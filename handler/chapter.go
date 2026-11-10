package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/erivelto/read-tracker/tracker/domain"
	"github.com/erivelto/read-tracker/tracker/usecase"
)

// ChapterUsecase defines the contract the handler uses for chapter operations.
type ChapterUsecase interface {
	List(ctx context.Context, filter domain.ChapterFilter) (usecase.ChapterPage, error)
	MarkRead(ctx context.Context, id bson.ObjectID) (*domain.ChapterView, error)
	UnreadCount(ctx context.Context, titleID *bson.ObjectID) (domain.UnreadCount, error)
}

// ChaptersEnvelope wraps the chapter list response.
type ChaptersEnvelope struct {
	Chapters   []domain.ChapterView `json:"chapters"`
	NextCursor string               `json:"nextCursor,omitempty"`
}

// ChapterHandler handles HTTP requests for chapter operations.
type ChapterHandler struct {
	usecase ChapterUsecase
}

// NewChapterHandler returns a new ChapterHandler.
func NewChapterHandler(uc ChapterUsecase) *ChapterHandler {
	return &ChapterHandler{usecase: uc}
}

// RegisterRoutes registers the chapter routes on the given RouterGroup.
func (h *ChapterHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/chapters", h.ListChapters)
	rg.PATCH("/chapters/:id/read", h.MarkChapterRead)
	rg.GET("/chapters/unread-count", h.UnreadCount)
}

// ListChapters handles GET /v1/chapters.
//
//	@Summary      List chapters
//	@Tags         chapters
//	@Produce      json
//	@Param        unread            query     bool    false  "Only unread chapters"
//	@Param        exclude_imported  query     bool    false  "Exclude chapters imported on a title's first crawl"
//	@Param        title_id          query     string  false  "Filter by title id"
//	@Param        host              query     string  false  "Filter by source host"
//	@Param        cursor            query     string  false  "Pagination cursor from a previous response"
//	@Param        limit             query     int     false  "Page size (1-200, default 50)"
//	@Success      200  {object}  ChaptersEnvelope
//	@Failure      400  {object}  ErrorEnvelope
//	@Failure      500  {object}  ErrorEnvelope
//	@Router       /chapters [get]
func (h *ChapterHandler) ListChapters(c *gin.Context) {
	filter := domain.ChapterFilter{}

	if rawUnread := c.Query("unread"); rawUnread != "" {
		unread := strings.EqualFold(rawUnread, "true")
		filter.Unread = &unread
	}
	filter.ExcludeImported = strings.EqualFold(c.Query("exclude_imported"), "true")

	if rawTitleID := c.Query("title_id"); rawTitleID != "" {
		titleID, err := bson.ObjectIDFromHex(rawTitleID)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid title_id", nil)
			return
		}
		filter.TitleID = &titleID
	}

	if rawHost := c.Query("host"); rawHost != "" {
		filter.Host = &rawHost
	}

	if rawCursor := c.Query("cursor"); rawCursor != "" {
		filter.Cursor = &rawCursor
	}

	limit, err := usecase.ParseLimit(c.Query("limit"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "BAD_REQUEST", err.Error(), nil)
		return
	}
	filter.Limit = limit

	page, err := h.usecase.List(c.Request.Context(), filter)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCursor) {
			RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid cursor", nil)
			return
		}
		RespondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, ChaptersEnvelope{Chapters: page.Chapters, NextCursor: page.NextCursor})
}

// MarkChapterRead handles PATCH /v1/chapters/:id/read.
//
//	@Summary      Mark a chapter as read
//	@Tags         chapters
//	@Produce      json
//	@Param        id   path      string  true  "Chapter ID"
//	@Success      200  {object}  domain.ChapterView
//	@Failure      400  {object}  ErrorEnvelope
//	@Failure      404  {object}  ErrorEnvelope
//	@Failure      500  {object}  ErrorEnvelope
//	@Router       /chapters/{id}/read [patch]
func (h *ChapterHandler) MarkChapterRead(c *gin.Context) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid chapter id", nil)
		return
	}

	chapter, err := h.usecase.MarkRead(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrChapterNotFound) {
			RespondError(c, http.StatusNotFound, "NOT_FOUND", "chapter not found", nil)
			return
		}
		RespondInternalError(c)
		return
	}

	RespondData(c, http.StatusOK, chapter)
}

// UnreadCount handles GET /v1/chapters/unread-count.
//
//	@Summary      Count unread chapters
//	@Tags         chapters
//	@Produce      json
//	@Param        title_id  query     string  false  "Scope the count to one title"
//	@Success      200       {object}  domain.UnreadCount
//	@Failure      400       {object}  ErrorEnvelope
//	@Failure      500       {object}  ErrorEnvelope
//	@Router       /chapters/unread-count [get]
func (h *ChapterHandler) UnreadCount(c *gin.Context) {
	var titleID *bson.ObjectID

	if rawTitleID := c.Query("title_id"); rawTitleID != "" {
		parsed, err := bson.ObjectIDFromHex(rawTitleID)
		if err != nil {
			RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid title_id", nil)
			return
		}
		titleID = &parsed
	}

	count, err := h.usecase.UnreadCount(c.Request.Context(), titleID)
	if err != nil {
		RespondInternalError(c)
		return
	}

	RespondData(c, http.StatusOK, count)
}
