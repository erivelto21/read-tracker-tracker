package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/erivelto/read-tracker/tracker/domain"
	"github.com/erivelto/read-tracker/tracker/handler"
	"github.com/erivelto/read-tracker/tracker/usecase"
)

// fakeChapterUsecase is a test double for handler.ChapterUsecase.
type fakeChapterUsecase struct {
	listFn  func(ctx context.Context, filter domain.ChapterFilter) (usecase.ChapterPage, error)
	readFn  func(ctx context.Context, id bson.ObjectID) (*domain.ChapterView, error)
	countFn func(ctx context.Context, titleID *bson.ObjectID) (domain.UnreadCount, error)
}

func (f *fakeChapterUsecase) List(ctx context.Context, filter domain.ChapterFilter) (usecase.ChapterPage, error) {
	return f.listFn(ctx, filter)
}

func (f *fakeChapterUsecase) MarkRead(ctx context.Context, id bson.ObjectID) (*domain.ChapterView, error) {
	return f.readFn(ctx, id)
}

func (f *fakeChapterUsecase) UnreadCount(ctx context.Context, titleID *bson.ObjectID) (domain.UnreadCount, error) {
	return f.countFn(ctx, titleID)
}

func newChapterRouter(uc handler.ChapterUsecase) *gin.Engine {
	router := gin.New()
	group := router.Group("/v1")
	handler.NewChapterHandler(uc).RegisterRoutes(group)
	return router
}

func TestListChaptersPassesFilters(t *testing.T) {
	var captured domain.ChapterFilter

	uc := &fakeChapterUsecase{
		listFn: func(_ context.Context, filter domain.ChapterFilter) (usecase.ChapterPage, error) {
			captured = filter
			return usecase.ChapterPage{Chapters: []domain.ChapterView{{ID: "c1", Name: "Chapter 1"}}}, nil
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/chapters?unread=true&exclude_imported=true&host=a.example.com&limit=10", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if captured.Unread == nil || !*captured.Unread {
		t.Fatal("the unread filter was not passed through")
	}
	if !captured.ExcludeImported {
		t.Fatal("the imported filter was not passed through")
	}
	if captured.Host == nil || *captured.Host != "a.example.com" {
		t.Fatalf("host filter = %v", captured.Host)
	}
	if captured.Limit != 10 {
		t.Fatalf("limit = %d, want 10", captured.Limit)
	}
}

func TestListChaptersRejectsInvalidTitleID(t *testing.T) {
	uc := &fakeChapterUsecase{
		listFn: func(context.Context, domain.ChapterFilter) (usecase.ChapterPage, error) {
			t.Fatal("the usecase must not be called with an invalid title_id")
			return usecase.ChapterPage{}, nil
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/chapters?title_id=not-an-object-id", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestListChaptersRejectsInvalidLimit(t *testing.T) {
	uc := &fakeChapterUsecase{
		listFn: func(context.Context, domain.ChapterFilter) (usecase.ChapterPage, error) {
			t.Fatal("the usecase must not be called with an invalid limit")
			return usecase.ChapterPage{}, nil
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/chapters?limit=abc", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestListChaptersRejectsInvalidCursor(t *testing.T) {
	uc := &fakeChapterUsecase{
		listFn: func(context.Context, domain.ChapterFilter) (usecase.ChapterPage, error) {
			return usecase.ChapterPage{}, domain.ErrInvalidCursor
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/chapters?cursor=garbage", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}

	var body handler.ErrorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "BAD_REQUEST" {
		t.Fatalf("error code = %q, want BAD_REQUEST", body.Error.Code)
	}
}

func TestListChaptersReturnsCursor(t *testing.T) {
	uc := &fakeChapterUsecase{
		listFn: func(context.Context, domain.ChapterFilter) (usecase.ChapterPage, error) {
			return usecase.ChapterPage{
				Chapters:   []domain.ChapterView{{ID: "c1"}},
				NextCursor: "next-page",
			}, nil
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/chapters", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	var body handler.ChaptersEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.NextCursor != "next-page" {
		t.Fatalf("NextCursor = %q, want next-page", body.NextCursor)
	}
}

func TestMarkChapterReadReturnsChapter(t *testing.T) {
	id := bson.NewObjectID()
	uc := &fakeChapterUsecase{
		readFn: func(_ context.Context, got bson.ObjectID) (*domain.ChapterView, error) {
			if got != id {
				t.Fatalf("id = %v, want %v", got, id)
			}
			return &domain.ChapterView{ID: id.Hex(), Name: "Chapter 1"}, nil
		},
	}

	request := httptest.NewRequest(http.MethodPatch, "/v1/chapters/"+id.Hex()+"/read", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestMarkChapterReadRejectsInvalidID(t *testing.T) {
	uc := &fakeChapterUsecase{
		readFn: func(context.Context, bson.ObjectID) (*domain.ChapterView, error) {
			t.Fatal("the usecase must not be called with an invalid id")
			return nil, nil
		},
	}

	request := httptest.NewRequest(http.MethodPatch, "/v1/chapters/nope/read", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestMarkChapterReadReturnsNotFound(t *testing.T) {
	uc := &fakeChapterUsecase{
		readFn: func(context.Context, bson.ObjectID) (*domain.ChapterView, error) {
			return nil, domain.ErrChapterNotFound
		},
	}

	request := httptest.NewRequest(http.MethodPatch, "/v1/chapters/"+bson.NewObjectID().Hex()+"/read", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
}

func TestUnreadCountReturnsBothCounts(t *testing.T) {
	uc := &fakeChapterUsecase{
		countFn: func(context.Context, *bson.ObjectID) (domain.UnreadCount, error) {
			return domain.UnreadCount{Total: 10, ExcludingImported: 4}, nil
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/chapters/unread-count", nil)
	recorder := httptest.NewRecorder()
	newChapterRouter(uc).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}

	var body handler.DataEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := body.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data shape: %T", body.Data)
	}
	if data["total"] != float64(10) || data["excludingImported"] != float64(4) {
		t.Fatalf("counts = %v", data)
	}
}
