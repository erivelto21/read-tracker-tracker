package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/erivelto/read-tracker/tracker/domain"
	"github.com/erivelto/read-tracker/tracker/handler"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeTitleUsecase is a test double for handler.TitleUsecase.
type fakeTitleUsecase struct {
	createFn func(ctx context.Context, title *domain.Title) (*domain.Title, error)
	listFn   func(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error)
	updateFn func(ctx context.Context, externalID string, fields domain.TitleUpdate) (*domain.Title, error)
	deleteFn func(ctx context.Context, externalID string) error
}

func (m *fakeTitleUsecase) Create(ctx context.Context, title *domain.Title) (*domain.Title, error) {
	return m.createFn(ctx, title)
}

func (m *fakeTitleUsecase) List(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error) {
	return m.listFn(ctx, filter)
}

func (m *fakeTitleUsecase) Update(ctx context.Context, externalID string, fields domain.TitleUpdate) (*domain.Title, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, externalID, fields)
	}
	return nil, nil
}

func (m *fakeTitleUsecase) Delete(ctx context.Context, externalID string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, externalID)
	}
	return nil
}

func newTestRouter(uc handler.TitleUsecase) *gin.Engine {
	r := gin.New()
	h := handler.NewTitleHandler(uc)
	h.RegisterRoutes(r.Group("/v1"))
	return r
}

func performRequest(router *gin.Engine, body interface{}) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/titles", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestCreateTitle(t *testing.T) {
	t.Parallel()

	chapterOne := 1
	pageOne := 10
	linkURL := "https://example.com"

	successUC := &fakeTitleUsecase{
		createFn: func(_ context.Context, t *domain.Title) (*domain.Title, error) {
			return t, nil
		},
	}
	conflictUC := &fakeTitleUsecase{
		createFn: func(_ context.Context, _ *domain.Title) (*domain.Title, error) {
			return nil, domain.ErrAlreadyExists
		},
	}

	tests := []struct {
		name          string
		uc            handler.TitleUsecase
		body          interface{}
		wantStatus    int
		wantCode      string
		wantDetailMsg string
	}{
		{
			name: "valid manga request",
			uc:   successUC,
			body: map[string]interface{}{
				"name": "Berserk", "type": "manga", "chapter": chapterOne, "link": linkURL,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid book request",
			uc:   successUC,
			body: map[string]interface{}{
				"name": "Dune", "type": "book", "chapter": chapterOne, "page": pageOne,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid novel request",
			uc:   successUC,
			body: map[string]interface{}{
				"name": "Overlord", "type": "novel", "chapter": chapterOne, "link": linkURL,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid article request (chapter not required)",
			uc:   successUC,
			body: map[string]interface{}{
				"name": "Some Article", "type": "article", "link": linkURL,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "valid manhua request",
			uc:   successUC,
			body: map[string]interface{}{
				"name": "Solo Leveling", "type": "manhua", "chapter": chapterOne, "link": linkURL,
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "missing name",
			uc:         successUC,
			body:       map[string]interface{}{"type": "manga", "chapter": chapterOne, "link": linkURL},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:       "chapter missing for manga",
			uc:         successUC,
			body:       map[string]interface{}{"name": "Berserk", "type": "manga", "link": linkURL},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:       "page missing for book",
			uc:         successUC,
			body:       map[string]interface{}{"name": "Dune", "type": "book", "chapter": chapterOne},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:       "link missing for novel",
			uc:         successUC,
			body:       map[string]interface{}{"name": "Overlord", "type": "novel", "chapter": chapterOne},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:       "invalid type enum",
			uc:         successUC,
			body:       map[string]interface{}{"name": "Something", "type": "comic"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name: "duplicate name returns 409",
			uc:   conflictUC,
			body: map[string]interface{}{
				"name": "Berserk", "type": "manga", "chapter": chapterOne, "link": linkURL,
			},
			wantStatus: http.StatusConflict,
			wantCode:   "CONFLICT",
		},
		{
			name:       "invalid link URL",
			uc:         successUC,
			body:       map[string]interface{}{"name": "Berserk", "type": "manga", "chapter": chapterOne, "link": "not-a-url"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "BAD_REQUEST",
		},
		{
			name:          "required field message is human-readable",
			uc:            successUC,
			body:          map[string]interface{}{"type": "manga", "chapter": chapterOne, "link": linkURL},
			wantStatus:    http.StatusBadRequest,
			wantCode:      "BAD_REQUEST",
			wantDetailMsg: "This field is required",
		},
		{
			name:          "oneof field message is human-readable",
			uc:            successUC,
			body:          map[string]interface{}{"name": "Something", "type": "comic"},
			wantStatus:    http.StatusBadRequest,
			wantCode:      "BAD_REQUEST",
			wantDetailMsg: "Valid values are: book, manga, manhua, novel and article",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newTestRouter(tt.uc)
			w := performRequest(router, tt.body)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantCode != "" {
				var errResp struct {
					Error struct {
						Code    string `json:"code"`
						Details []struct {
							Message string `json:"message"`
						} `json:"details"`
					} `json:"error"`
				}
				if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if errResp.Error.Code != tt.wantCode {
					t.Errorf("error.code = %q, want %q", errResp.Error.Code, tt.wantCode)
				}
				if tt.wantDetailMsg != "" {
					found := false
					for _, d := range errResp.Error.Details {
						if d.Message == tt.wantDetailMsg {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("no detail with message %q; got details: %+v", tt.wantDetailMsg, errResp.Error.Details)
					}
				}
			}
		})
	}
}


func performGetRequest(router *gin.Engine, url string) *httptest.ResponseRecorder {
req := httptest.NewRequest(http.MethodGet, url, nil)
w := httptest.NewRecorder()
router.ServeHTTP(w, req)
return w
}

func TestListTitles(t *testing.T) {
t.Parallel()

allTitles := []domain.Title{
{ExternalID: "abc", Name: "Berserk", Type: domain.Manga},
{ExternalID: "def", Name: "Dune", Type: domain.Book},
}

tests := []struct {
name        string
url         string
listFn      func(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error)
wantStatus  int
wantCode    string
wantDataLen int
}{
{
name: "returns all titles when no filters",
url:  "/v1/titles",
listFn: func(_ context.Context, f domain.TitleFilter) ([]domain.Title, error) {
if f.Type != nil || f.Name != nil {
t.Error("expected no filter, got one")
}
return allTitles, nil
},
wantStatus:  http.StatusOK,
wantDataLen: 2,
},
{
name: "filters by type (lowercase)",
url:  "/v1/titles?type=manga",
listFn: func(_ context.Context, f domain.TitleFilter) ([]domain.Title, error) {
if f.Type == nil || *f.Type != domain.Manga {
t.Errorf("expected type=manga, got %v", f.Type)
}
return allTitles[:1], nil
},
wantStatus:  http.StatusOK,
wantDataLen: 1,
},
{
name: "normalizes type to lowercase (MANGA -> manga)",
url:  "/v1/titles?type=MANGA",
listFn: func(_ context.Context, f domain.TitleFilter) ([]domain.Title, error) {
if f.Type == nil || *f.Type != domain.Manga {
t.Errorf("expected normalized type=manga, got %v", f.Type)
}
return allTitles[:1], nil
},
wantStatus:  http.StatusOK,
wantDataLen: 1,
},
{
name: "filters by name",
url:  "/v1/titles?name=Berserk",
listFn: func(_ context.Context, f domain.TitleFilter) ([]domain.Title, error) {
if f.Name == nil || *f.Name != "Berserk" {
t.Errorf("expected name=Berserk, got %v", f.Name)
}
return allTitles[:1], nil
},
wantStatus:  http.StatusOK,
wantDataLen: 1,
},
{
name: "invalid type returns 400",
url:  "/v1/titles?type=INVALID",
listFn: func(_ context.Context, _ domain.TitleFilter) ([]domain.Title, error) {
t.Error("listFn should not be called on bad request")
return nil, nil
},
wantStatus: http.StatusBadRequest,
wantCode:   "BAD_REQUEST",
},
{
name: "empty type param is ignored",
url:  "/v1/titles?type=",
listFn: func(_ context.Context, f domain.TitleFilter) ([]domain.Title, error) {
if f.Type != nil {
t.Errorf("expected nil type filter, got %v", f.Type)
}
return allTitles, nil
},
wantStatus:  http.StatusOK,
wantDataLen: 2,
},
{
name: "empty name param is ignored",
url:  "/v1/titles?name=",
listFn: func(_ context.Context, f domain.TitleFilter) ([]domain.Title, error) {
if f.Name != nil {
t.Errorf("expected nil name filter, got %v", f.Name)
}
return allTitles, nil
},
wantStatus:  http.StatusOK,
wantDataLen: 2,
},
{
name: "no results returns 200 with empty data array",
url:  "/v1/titles",
listFn: func(_ context.Context, _ domain.TitleFilter) ([]domain.Title, error) {
return []domain.Title{}, nil
},
wantStatus:  http.StatusOK,
wantDataLen: 0,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
t.Parallel()

uc := &fakeTitleUsecase{
createFn: func(_ context.Context, t *domain.Title) (*domain.Title, error) {
return t, nil
},
listFn: tt.listFn,
}
router := newTestRouter(uc)
w := performGetRequest(router, tt.url)

if w.Code != tt.wantStatus {
t.Errorf("status = %d, want %d; body: %s", w.Code, tt.wantStatus, w.Body.String())
}

if tt.wantCode != "" {
var errResp struct {
Error struct {
Code string `json:"code"`
} `json:"error"`
}
if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
t.Fatalf("failed to parse error response: %v", err)
}
if errResp.Error.Code != tt.wantCode {
t.Errorf("error.code = %q, want %q", errResp.Error.Code, tt.wantCode)
}
}

if tt.wantStatus == http.StatusOK {
var resp struct {
Titles []domain.Title `json:"titles"`
}
if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
t.Fatalf("failed to parse response: %v", err)
}
if len(resp.Titles) != tt.wantDataLen {
t.Errorf("titles length = %d, want %d", len(resp.Titles), tt.wantDataLen)
}
}
})
}
}

func performPatchRequest(router *gin.Engine, id string, body interface{}) *httptest.ResponseRecorder {
b, _ := json.Marshal(body)
req := httptest.NewRequest(http.MethodPatch, "/v1/titles/"+id, bytes.NewReader(b))
req.Header.Set("Content-Type", "application/json")
w := httptest.NewRecorder()
router.ServeHTTP(w, req)
return w
}

func TestUpdateTitle(t *testing.T) {
t.Parallel()

chapter := 5
existing := &domain.Title{ExternalID: "uuid-1", Name: "Berserk", Type: domain.Manga, Chapter: &chapter}

tests := []struct {
name       string
id         string
body       interface{}
updateFn   func(ctx context.Context, id string, fields domain.TitleUpdate) (*domain.Title, error)
wantStatus int
wantCode   string
}{
{
name: "success - update chapter",
id:   "uuid-1",
body: map[string]interface{}{"chapter": 10},
updateFn: func(_ context.Context, _ string, _ domain.TitleUpdate) (*domain.Title, error) {
	return existing, nil
},
wantStatus: http.StatusOK,
},
{
name:       "empty body returns 400",
id:         "uuid-1",
body:       map[string]interface{}{},
wantStatus: http.StatusBadRequest,
wantCode:   "BAD_REQUEST",
},
{
name:       "chapter below minimum returns 400",
id:         "uuid-1",
body:       map[string]interface{}{"chapter": -200},
wantStatus: http.StatusBadRequest,
wantCode:   "BAD_REQUEST",
},
{
name:       "page above maximum returns 400",
id:         "uuid-1",
body:       map[string]interface{}{"page": 99999},
wantStatus: http.StatusBadRequest,
wantCode:   "BAD_REQUEST",
},
{
name:       "invalid link URL returns 400",
id:         "uuid-1",
body:       map[string]interface{}{"link": "not-a-url"},
wantStatus: http.StatusBadRequest,
wantCode:   "BAD_REQUEST",
},
{
name: "title not found returns 404",
id:   "missing-id",
body: map[string]interface{}{"chapter": 1},
updateFn: func(_ context.Context, _ string, _ domain.TitleUpdate) (*domain.Title, error) {
	return nil, domain.ErrNotFound
},
wantStatus: http.StatusNotFound,
wantCode:   "NOT_FOUND",
},
{
name: "internal error returns 500",
id:   "uuid-1",
body: map[string]interface{}{"chapter": 1},
updateFn: func(_ context.Context, _ string, _ domain.TitleUpdate) (*domain.Title, error) {
	return nil, errors.New("unexpected db failure")
},
wantStatus: http.StatusInternalServerError,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
t.Parallel()

uc := &fakeTitleUsecase{
	createFn: func(_ context.Context, t *domain.Title) (*domain.Title, error) { return t, nil },
	listFn:   func(_ context.Context, _ domain.TitleFilter) ([]domain.Title, error) { return nil, nil },
	updateFn: tt.updateFn,
}
router := newTestRouter(uc)
w := performPatchRequest(router, tt.id, tt.body)

if w.Code != tt.wantStatus {
	t.Errorf("status = %d, want %d; body: %s", w.Code, tt.wantStatus, w.Body.String())
}

if tt.wantCode != "" {
	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to parse error response: %v", err)
	}
	if errResp.Error.Code != tt.wantCode {
		t.Errorf("error.code = %q, want %q", errResp.Error.Code, tt.wantCode)
	}
}
})
}
}

func performDeleteRequest(router *gin.Engine, id string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, "/v1/titles/"+id, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestDeleteTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		id         string
		deleteFn   func(ctx context.Context, externalID string) error
		wantStatus int
	}{
		{
			name: "success returns 204",
			id:   "uuid-1",
			deleteFn: func(_ context.Context, _ string) error {
				return nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "non-existent title returns 204",
			id:   "missing-id",
			deleteFn: func(_ context.Context, _ string) error {
				return nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "internal error returns 500",
			id:   "uuid-1",
			deleteFn: func(_ context.Context, _ string) error {
				return errors.New("unexpected db failure")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := &fakeTitleUsecase{
				createFn: func(_ context.Context, t *domain.Title) (*domain.Title, error) { return t, nil },
				listFn:   func(_ context.Context, _ domain.TitleFilter) ([]domain.Title, error) { return nil, nil },
				deleteFn: tt.deleteFn,
			}
			router := newTestRouter(uc)
			w := performDeleteRequest(router, tt.id)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
