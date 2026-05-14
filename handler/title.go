package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// TitleUsecase defines the contract the handler uses for title operations.
type TitleUsecase interface {
	Create(ctx context.Context, title *domain.Title) (*domain.Title, error)
	List(ctx context.Context, filter domain.TitleFilter) ([]domain.Title, error)
}

// CreateTitleRequest is the JSON request body for POST /titles.
// Pointer types allow the validator to distinguish "not provided" from zero values.
type CreateTitleRequest struct {
	Name string           `json:"name" validate:"required,min=3,max=100"`
	Type domain.TitleType `json:"type" validate:"required,oneof=book manga manhua novel article"`
	// chapter is required for book, manga, manhua, novel; optional for article
	Chapter *int `json:"chapter" validate:"required_if=Type book,required_if=Type manga,required_if=Type manhua,required_if=Type novel,omitempty,min=-100,max=10000"`
	// page is required only for book
	Page *int `json:"page" validate:"required_if=Type book,omitempty,min=0,max=10000"`
	// link is required for manga, manhua, novel
	Link        *string `json:"link"        validate:"required_if=Type manga,required_if=Type manhua,required_if=Type novel,omitempty,http_url,max=200"`
	Observation *string `json:"observation" validate:"omitempty,max=500"`
}

// validationMessage translates a validator.FieldError tag into a human-readable message.
func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required", "required_if":
		return "This field is required"
	case "oneof":
		values := strings.Fields(fe.Param())
		if len(values) == 0 {
			return "Invalid value"
		}
		if len(values) == 1 {
			return "Valid values are: " + values[0]
		}
		return "Valid values are: " + strings.Join(values[:len(values)-1], ", ") + " and " + values[len(values)-1]
	case "min":
		return "Minimum value is " + fe.Param()
	case "max":
		return "Maximum value is " + fe.Param()
	case "http_url":
		return "Must be a valid URL"
	default:
		return "Invalid value"
	}
}

// TitleHandler handles HTTP requests for title operations.
type TitleHandler struct {
	usecase  TitleUsecase
	validate *validator.Validate
}

// NewTitleHandler returns a new TitleHandler.
func NewTitleHandler(uc TitleUsecase) *TitleHandler {
	return &TitleHandler{
		usecase:  uc,
		validate: validator.New(),
	}
}

// RegisterRoutes registers the title routes on the given RouterGroup.
func (h *TitleHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/titles", h.CreateTitle)
	rg.GET("/titles", h.ListTitles)
}

// CreateTitle handles POST /titles.
//
//	@Summary      Create a title
//	@Tags         titles
//	@Accept       json
//	@Produce      json
//	@Param        title  body      CreateTitleRequest  true  "Title to create"
//	@Success      201    {object}  domain.Title
//	@Failure      400    {object}  ErrorEnvelope
//	@Failure      409    {object}  ErrorEnvelope
//	@Failure      500    {object}  ErrorEnvelope
//	@Router       /titles [post]
func (h *TitleHandler) CreateTitle(c *gin.Context) {
	var req CreateTitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "Bad Request", nil)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			details := make([]ErrorDetail, 0, len(ve))
			for _, fe := range ve {
				details = append(details, ErrorDetail{
					Field:   fe.Field(),
					Message: validationMessage(fe),
				})
			}
			RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "Bad Request", details)
			return
		}
		RespondInternalError(c)
		return
	}

	title := &domain.Title{
		Name:        req.Name,
		Type:        req.Type,
		Chapter:     req.Chapter,
		Page:        req.Page,
		Link:        req.Link,
		Observation: req.Observation,
	}

	created, err := h.usecase.Create(c.Request.Context(), title)
	if err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			RespondError(c, http.StatusConflict, "CONFLICT", "a title with this name already exists", nil)
			return
		}
		RespondInternalError(c)
		return
	}

	RespondData(c, http.StatusCreated, created)
}

// ListTitles handles GET /titles.
//
//	@Summary      List titles
//	@Tags         titles
//	@Produce      json
//	@Param        type  query     string  false  "Filter by type (book, manga, manhua, novel, article)"
//	@Param        name  query     string  false  "Filter by name (partial, case-insensitive)"
//	@Success      200   {object}  DataEnvelope
//	@Failure      400   {object}  ErrorEnvelope
//	@Failure      500   {object}  ErrorEnvelope
//	@Router       /titles [get]
func (h *TitleHandler) ListTitles(c *gin.Context) {
	var filter domain.TitleFilter

	if rawType := strings.ToLower(c.Query("type")); rawType != "" {
		t := domain.TitleType(rawType)
		switch t {
		case domain.Book, domain.Manga, domain.Manhua, domain.Novel, domain.Article:
			filter.Type = &t
		default:
			RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid type value", nil)
			return
		}
	}

	if rawName := c.Query("name"); rawName != "" {
		filter.Name = &rawName
	}

	titles, err := h.usecase.List(c.Request.Context(), filter)
	if err != nil {
		RespondInternalError(c)
		return
	}

	RespondData(c, http.StatusOK, titles)
}
