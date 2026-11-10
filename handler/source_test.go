package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/erivelto/read-tracker/tracker/domain"
	"github.com/erivelto/read-tracker/tracker/handler"
)

// fakeSourceUsecase is a test double for handler.SourceUsecase.
type fakeSourceUsecase struct {
	listFn   func(ctx context.Context) ([]domain.Source, error)
	getFn    func(ctx context.Context, id bson.ObjectID) (*domain.Source, error)
	createFn func(ctx context.Context, source domain.Source, by string) (*domain.Source, error)
	updateFn func(ctx context.Context, id bson.ObjectID, patch domain.SourcePatch, by string) (*domain.Source, error)
	deleteFn func(ctx context.Context, id bson.ObjectID) error
}

func (f *fakeSourceUsecase) List(ctx context.Context) ([]domain.Source, error) {
	return f.listFn(ctx)
}

func (f *fakeSourceUsecase) Get(ctx context.Context, id bson.ObjectID) (*domain.Source, error) {
	return f.getFn(ctx, id)
}

func (f *fakeSourceUsecase) Create(ctx context.Context, source domain.Source, by string) (*domain.Source, error) {
	return f.createFn(ctx, source, by)
}

func (f *fakeSourceUsecase) Update(
	ctx context.Context, id bson.ObjectID, patch domain.SourcePatch, by string,
) (*domain.Source, error) {
	return f.updateFn(ctx, id, patch, by)
}

func (f *fakeSourceUsecase) Delete(ctx context.Context, id bson.ObjectID) error {
	return f.deleteFn(ctx, id)
}

func newSourceRouter(uc handler.SourceUsecase) *gin.Engine {
	r := gin.New()
	h := handler.NewSourceHandler(uc)
	h.RegisterRoutes(r.Group("/v1"))
	return r
}

func performSourceRequest(
	router *gin.Engine, method, path string, body interface{},
) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func validCreateBody() map[string]any {
	return map[string]any{
		"host":       "example.com",
		"name":       "Example",
		"urlPattern": `^https://example\.com/manga/[^/]+$`,
		"strategy":   "feed",
		"profile":    "chrome",
		"feedUrl":    "https://example.com/feed.xml",
		"enabled":    true,
	}
}

func TestCreateSource(t *testing.T) {
	t.Parallel()

	var received domain.Source
	uc := &fakeSourceUsecase{
		createFn: func(_ context.Context, source domain.Source, by string) (*domain.Source, error) {
			received = source
			source.ID = bson.NewObjectID()
			return &source, nil
		},
	}

	w := performSourceRequest(newSourceRouter(uc), http.MethodPost, "/v1/sources", validCreateBody())

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if received.Host != "example.com" {
		t.Errorf("expected the host to reach the usecase, got %q", received.Host)
	}
	if received.Strategy != domain.SourceStrategyFeed {
		t.Errorf("expected the feed strategy, got %q", received.Strategy)
	}
}

func TestCreateSourceValidationErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(map[string]any)
		wantKey string
	}{
		{
			name:    "missing host",
			mutate:  func(b map[string]any) { delete(b, "host") },
			wantKey: "host",
		},
		{
			name:    "missing url pattern",
			mutate:  func(b map[string]any) { delete(b, "urlPattern") },
			wantKey: "urlPattern",
		},
		{
			name:    "unknown strategy",
			mutate:  func(b map[string]any) { b["strategy"] = "xpath" },
			wantKey: "strategy",
		},
		{
			name:    "invalid list url",
			mutate:  func(b map[string]any) { b["listUrl"] = "not-a-url" },
			wantKey: "listUrl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := &fakeSourceUsecase{
				createFn: func(context.Context, domain.Source, string) (*domain.Source, error) {
					t.Fatal("an invalid request must not reach the usecase")
					return nil, nil
				},
			}

			body := validCreateBody()
			tt.mutate(body)

			w := performSourceRequest(newSourceRouter(uc), http.MethodPost, "/v1/sources", body)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
			}

			var envelope handler.ErrorEnvelope
			if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decoding the error envelope: %v", err)
			}
			if len(envelope.Error.Details) == 0 {
				t.Fatal("expected field-level details")
			}
			if envelope.Error.Details[0].Field != tt.wantKey {
				t.Errorf("expected the detail to name %q, got %q", tt.wantKey, envelope.Error.Details[0].Field)
			}
		})
	}
}

func TestCreateSourceConfigErrorIsAFieldError(t *testing.T) {
	t.Parallel()

	uc := &fakeSourceUsecase{
		createFn: func(context.Context, domain.Source, string) (*domain.Source, error) {
			return nil, &domain.SourceConfigError{Field: "feedUrl", Reason: "is required for the feed strategy"}
		},
	}

	w := performSourceRequest(newSourceRouter(uc), http.MethodPost, "/v1/sources", validCreateBody())

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var envelope handler.ErrorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decoding the error envelope: %v", err)
	}
	if len(envelope.Error.Details) != 1 || envelope.Error.Details[0].Field != "feedUrl" {
		t.Fatalf("expected a feedUrl detail, got %+v", envelope.Error.Details)
	}
}

func TestCreateSourceDuplicateIsAConflict(t *testing.T) {
	t.Parallel()

	uc := &fakeSourceUsecase{
		createFn: func(context.Context, domain.Source, string) (*domain.Source, error) {
			return nil, domain.ErrSourceExists
		},
	}

	w := performSourceRequest(newSourceRouter(uc), http.MethodPost, "/v1/sources", validCreateBody())

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSourceRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()

	uc := &fakeSourceUsecase{
		updateFn: func(context.Context, bson.ObjectID, domain.SourcePatch, string) (*domain.Source, error) {
			t.Fatal("an empty patch must not reach the usecase")
			return nil, nil
		},
	}

	w := performSourceRequest(
		newSourceRouter(uc), http.MethodPatch, "/v1/sources/"+bson.NewObjectID().Hex(), map[string]any{},
	)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSource(t *testing.T) {
	t.Parallel()

	var receivedPatch domain.SourcePatch
	uc := &fakeSourceUsecase{
		updateFn: func(_ context.Context, id bson.ObjectID, patch domain.SourcePatch, _ string) (*domain.Source, error) {
			receivedPatch = patch
			return &domain.Source{ID: id, Host: "example.com"}, nil
		},
	}

	enabled := false
	w := performSourceRequest(
		newSourceRouter(uc),
		http.MethodPatch,
		"/v1/sources/"+bson.NewObjectID().Hex(),
		map[string]any{"enabled": enabled},
	)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if receivedPatch.Enabled == nil || *receivedPatch.Enabled {
		t.Error("expected the enabled field to be carried through")
	}
	// An absent field must stay nil so the repository does not overwrite it.
	if receivedPatch.Host != nil {
		t.Error("expected an absent host to remain unset")
	}
}

func TestSourceRoutesRejectAMalformedID(t *testing.T) {
	t.Parallel()

	uc := &fakeSourceUsecase{
		getFn: func(context.Context, bson.ObjectID) (*domain.Source, error) {
			t.Fatal("a malformed id must not reach the usecase")
			return nil, nil
		},
		deleteFn: func(context.Context, bson.ObjectID) error {
			t.Fatal("a malformed id must not reach the usecase")
			return nil
		},
	}

	router := newSourceRouter(uc)

	for _, tc := range []struct {
		method string
		body   any
	}{
		{http.MethodGet, nil},
		{http.MethodDelete, nil},
		{http.MethodPatch, map[string]any{"enabled": true}},
	} {
		w := performSourceRequest(router, tc.method, "/v1/sources/not-an-id", tc.body)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s with a malformed id: expected 400, got %d", tc.method, w.Code)
		}
	}
}

func TestDeleteSource(t *testing.T) {
	t.Parallel()

	uc := &fakeSourceUsecase{
		deleteFn: func(context.Context, bson.ObjectID) error { return nil },
	}

	w := performSourceRequest(
		newSourceRouter(uc), http.MethodDelete, "/v1/sources/"+bson.NewObjectID().Hex(), nil,
	)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteMissingSourceIsNotFound(t *testing.T) {
	t.Parallel()

	uc := &fakeSourceUsecase{
		deleteFn: func(context.Context, bson.ObjectID) error { return domain.ErrSourceNotFound },
	}

	w := performSourceRequest(
		newSourceRouter(uc), http.MethodDelete, "/v1/sources/"+bson.NewObjectID().Hex(), nil,
	)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListSources(t *testing.T) {
	t.Parallel()

	uc := &fakeSourceUsecase{
		listFn: func(context.Context) ([]domain.Source, error) {
			return []domain.Source{
				{Host: "a.example.com", Name: "A", Failing: true, ConsecutiveFailures: 4},
				{Host: "b.example.com", Name: "B"},
			}, nil
		},
	}

	w := performSourceRequest(newSourceRouter(uc), http.MethodGet, "/v1/sources", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var envelope handler.SourcesEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decoding the sources envelope: %v", err)
	}
	if len(envelope.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(envelope.Sources))
	}
	if !envelope.Sources[0].Failing || envelope.Sources[0].ConsecutiveFailures != 4 {
		t.Errorf("expected the health fields to be surfaced, got %+v", envelope.Sources[0])
	}
}
