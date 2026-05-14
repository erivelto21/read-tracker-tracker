package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
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

// mockTitleUsecase is a test double for handler.TitleUsecase.
type mockTitleUsecase struct {
	createFn func(ctx context.Context, title *domain.Title) (*domain.Title, error)
}

func (m *mockTitleUsecase) Create(ctx context.Context, title *domain.Title) (*domain.Title, error) {
	return m.createFn(ctx, title)
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

	successUC := &mockTitleUsecase{
		createFn: func(_ context.Context, t *domain.Title) (*domain.Title, error) {
			return t, nil
		},
	}
	conflictUC := &mockTitleUsecase{
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
				"name": "Some Article", "type": "article",
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
