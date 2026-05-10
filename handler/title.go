package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// TitleUsecase defines the contract the handler uses for title operations.
type TitleUsecase interface {
	Create(ctx context.Context, title *domain.Title) (*domain.Title, error)
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
	Link        *string `json:"link"        validate:"required_if=Type manga,required_if=Type manhua,required_if=Type novel,omitempty,max=200"`
	Observation *string `json:"observation" validate:"omitempty,max=500"`
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
}

// CreateTitle handles POST /titles.
func (h *TitleHandler) CreateTitle(c *gin.Context) {
	var req CreateTitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "request validation failed", nil)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			details := make([]ErrorDetail, 0, len(ve))
			for _, fe := range ve {
				details = append(details, ErrorDetail{
					Field:   fe.Field(),
					Message: fmt.Sprintf("failed on '%s' validation", fe.Tag()),
				})
			}
			RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "request validation failed", details)
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
