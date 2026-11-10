package handler

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/erivelto/read-tracker/tracker/domain"
)

// SourceUsecase defines the contract the handler uses for source management.
type SourceUsecase interface {
	List(ctx context.Context) ([]domain.Source, error)
	Get(ctx context.Context, id bson.ObjectID) (*domain.Source, error)
	Create(ctx context.Context, source domain.Source, by string) (*domain.Source, error)
	Update(ctx context.Context, id bson.ObjectID, patch domain.SourcePatch, by string) (*domain.Source, error)
	Delete(ctx context.Context, id bson.ObjectID) error
}

// SourcesEnvelope wraps the source list response.
type SourcesEnvelope struct {
	Sources []domain.Source `json:"sources"`
}

// updatedByAPI is the provenance recorded on every write. The API has no authentication, so the
// only honest value is the component that made the change; a per-user identity would be a lie
// until there is something to derive it from.
const updatedByAPI = "tracker-api"

// CreateSourceRequest is the JSON request body for POST /sources.
//
// The fields a strategy requires are checked by domain.ValidateSource rather than by tags, so
// the rules live in one place instead of being split between the validator and the usecase where
// the two could drift apart.
type CreateSourceRequest struct {
	Host          string                `json:"host"          validate:"required,max=255"`
	Name          string                `json:"name"          validate:"required,max=120"`
	URLPattern    string                `json:"urlPattern"    validate:"required,max=500"`
	Priority      int                   `json:"priority"      validate:"min=0,max=1000"`
	Strategy      domain.SourceStrategy `json:"strategy"      validate:"required,oneof=feed css json"`
	Enabled       bool                  `json:"enabled"`
	DelayMS       int                   `json:"delayMs"       validate:"min=0,max=60000"`
	Profile       string                `json:"profile"       validate:"required,max=60"`
	ListURL       string                `json:"listUrl"       validate:"omitempty,url,max=500"`
	FeedURL       string                `json:"feedUrl"       validate:"omitempty,url,max=500"`
	JSONURL       string                `json:"jsonUrl"       validate:"omitempty,url,max=500"`
	JSONPath      string                `json:"jsonPath"      validate:"max=500"`
	ItemSelector  string                `json:"itemSelector"  validate:"max=500"`
	TitleSelector string                `json:"titleSelector" validate:"max=500"`
	LinkAttr      string                `json:"linkAttr"      validate:"max=100"`
	DateSelector  string                `json:"dateSelector"  validate:"max=500"`
	NumberPattern string                `json:"numberPattern" validate:"max=500"`
}

// UpdateSourceRequest is the JSON request body for PATCH /sources/:id. Every field is optional;
// at least one must be provided.
type UpdateSourceRequest struct {
	Host          *string                `json:"host"          validate:"omitempty,max=255"`
	Name          *string                `json:"name"          validate:"omitempty,max=120"`
	URLPattern    *string                `json:"urlPattern"    validate:"omitempty,max=500"`
	Priority      *int                   `json:"priority"      validate:"omitempty,min=0,max=1000"`
	Strategy      *domain.SourceStrategy `json:"strategy"      validate:"omitempty,oneof=feed css json"`
	Enabled       *bool                  `json:"enabled"`
	DelayMS       *int                   `json:"delayMs"       validate:"omitempty,min=0,max=60000"`
	Profile       *string                `json:"profile"       validate:"omitempty,max=60"`
	ListURL       *string                `json:"listUrl"       validate:"omitempty,url,max=500"`
	FeedURL       *string                `json:"feedUrl"       validate:"omitempty,url,max=500"`
	JSONURL       *string                `json:"jsonUrl"       validate:"omitempty,url,max=500"`
	JSONPath      *string                `json:"jsonPath"      validate:"omitempty,max=500"`
	ItemSelector  *string                `json:"itemSelector"  validate:"omitempty,max=500"`
	TitleSelector *string                `json:"titleSelector" validate:"omitempty,max=500"`
	LinkAttr      *string                `json:"linkAttr"      validate:"omitempty,max=100"`
	DateSelector  *string                `json:"dateSelector"  validate:"omitempty,max=500"`
	NumberPattern *string                `json:"numberPattern" validate:"omitempty,max=500"`
}

// patch converts the request into a domain patch.
func (r UpdateSourceRequest) patch() domain.SourcePatch {
	return domain.SourcePatch{
		Host:          r.Host,
		Name:          r.Name,
		URLPattern:    r.URLPattern,
		Priority:      r.Priority,
		Strategy:      r.Strategy,
		Enabled:       r.Enabled,
		DelayMS:       r.DelayMS,
		Profile:       r.Profile,
		ListURL:       r.ListURL,
		FeedURL:       r.FeedURL,
		JSONURL:       r.JSONURL,
		JSONPath:      r.JSONPath,
		ItemSelector:  r.ItemSelector,
		TitleSelector: r.TitleSelector,
		LinkAttr:      r.LinkAttr,
		DateSelector:  r.DateSelector,
		NumberPattern: r.NumberPattern,
	}
}

// SourceHandler handles HTTP requests for source management.
type SourceHandler struct {
	usecase  SourceUsecase
	validate *validator.Validate
}

// NewSourceHandler returns a new SourceHandler.
func NewSourceHandler(uc SourceUsecase) *SourceHandler {
	return &SourceHandler{usecase: uc, validate: validator.New()}
}

// RegisterRoutes registers the source routes on the given RouterGroup.
func (h *SourceHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/sources", h.ListSources)
	rg.POST("/sources", h.CreateSource)
	rg.GET("/sources/:id", h.GetSource)
	rg.PATCH("/sources/:id", h.UpdateSource)
	rg.DELETE("/sources/:id", h.DeleteSource)
}

// ListSources handles GET /v1/sources.
//
//	@Summary      List sources
//	@Tags         sources
//	@Produce      json
//	@Success      200  {object}  SourcesEnvelope
//	@Failure      500  {object}  ErrorEnvelope
//	@Router       /sources [get]
func (h *SourceHandler) ListSources(c *gin.Context) {
	sources, err := h.usecase.List(c.Request.Context())
	if err != nil {
		RespondInternalError(c)
		return
	}

	c.JSON(http.StatusOK, SourcesEnvelope{Sources: sources})
}

// GetSource handles GET /v1/sources/:id.
//
//	@Summary      Get a source
//	@Tags         sources
//	@Produce      json
//	@Param        id   path      string  true  "Source ID"
//	@Success      200  {object}  domain.Source
//	@Failure      400  {object}  ErrorEnvelope
//	@Failure      404  {object}  ErrorEnvelope
//	@Failure      500  {object}  ErrorEnvelope
//	@Router       /sources/{id} [get]
func (h *SourceHandler) GetSource(c *gin.Context) {
	id, ok := parseSourceID(c)
	if !ok {
		return
	}

	source, err := h.usecase.Get(c.Request.Context(), id)
	if err != nil {
		h.respondSourceError(c, err)
		return
	}

	RespondData(c, http.StatusOK, source)
}

// CreateSource handles POST /v1/sources.
//
//	@Summary      Create a source
//	@Tags         sources
//	@Accept       json
//	@Produce      json
//	@Param        source  body      CreateSourceRequest  true  "Source to create"
//	@Success      201     {object}  domain.Source
//	@Failure      400     {object}  ErrorEnvelope
//	@Failure      409     {object}  ErrorEnvelope
//	@Failure      500     {object}  ErrorEnvelope
//	@Router       /sources [post]
func (h *SourceHandler) CreateSource(c *gin.Context) {
	var req CreateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "Bad Request", nil)
		return
	}

	if !h.validateRequest(c, req) {
		return
	}

	source := domain.Source{
		Host:          req.Host,
		Name:          req.Name,
		URLPattern:    req.URLPattern,
		Priority:      req.Priority,
		Strategy:      req.Strategy,
		Enabled:       req.Enabled,
		DelayMS:       req.DelayMS,
		Profile:       req.Profile,
		ListURL:       req.ListURL,
		FeedURL:       req.FeedURL,
		JSONURL:       req.JSONURL,
		JSONPath:      req.JSONPath,
		ItemSelector:  req.ItemSelector,
		TitleSelector: req.TitleSelector,
		LinkAttr:      req.LinkAttr,
		DateSelector:  req.DateSelector,
		NumberPattern: req.NumberPattern,
	}

	created, err := h.usecase.Create(c.Request.Context(), source, updatedByAPI)
	if err != nil {
		h.respondSourceError(c, err)
		return
	}

	RespondData(c, http.StatusCreated, created)
}

// UpdateSource handles PATCH /v1/sources/:id.
//
//	@Summary      Update a source
//	@Tags         sources
//	@Accept       json
//	@Produce      json
//	@Param        id      path      string               true  "Source ID"
//	@Param        source  body      UpdateSourceRequest  true  "Fields to update"
//	@Success      200     {object}  domain.Source
//	@Failure      400     {object}  ErrorEnvelope
//	@Failure      404     {object}  ErrorEnvelope
//	@Failure      409     {object}  ErrorEnvelope
//	@Failure      500     {object}  ErrorEnvelope
//	@Router       /sources/{id} [patch]
func (h *SourceHandler) UpdateSource(c *gin.Context) {
	id, ok := parseSourceID(c)
	if !ok {
		return
	}

	var req UpdateSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "Bad Request", nil)
		return
	}

	patch := req.patch()
	if patch.IsEmpty() {
		RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "at least one field must be provided", nil)
		return
	}

	if !h.validateRequest(c, req) {
		return
	}

	updated, err := h.usecase.Update(c.Request.Context(), id, patch, updatedByAPI)
	if err != nil {
		h.respondSourceError(c, err)
		return
	}

	RespondData(c, http.StatusOK, updated)
}

// DeleteSource handles DELETE /v1/sources/:id.
//
//	@Summary      Delete a source
//	@Tags         sources
//	@Param        id   path      string  true  "Source ID"
//	@Success      204
//	@Failure      400  {object}  ErrorEnvelope
//	@Failure      404  {object}  ErrorEnvelope
//	@Failure      500  {object}  ErrorEnvelope
//	@Router       /sources/{id} [delete]
func (h *SourceHandler) DeleteSource(c *gin.Context) {
	id, ok := parseSourceID(c)
	if !ok {
		return
	}

	if err := h.usecase.Delete(c.Request.Context(), id); err != nil {
		h.respondSourceError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// validateRequest runs the struct tags and writes a 400 with field-level details on failure.
func (h *SourceHandler) validateRequest(c *gin.Context, req any) bool {
	if err := h.validate.Struct(req); err != nil {
		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			details := make([]ErrorDetail, 0, len(ve))
			for _, fe := range ve {
				details = append(details, ErrorDetail{
					Field:   jsonFieldName(req, fe.StructField()),
					Message: validationMessage(fe),
				})
			}
			RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "Bad Request", details)
			return false
		}
		RespondInternalError(c)
		return false
	}
	return true
}

// jsonFieldName returns the JSON key a struct field is encoded as.
//
// The validator reports the Go field name ("FeedURL"), but the client knows the field by its
// JSON key ("feedUrl"). Sending the Go name would leave the client to guess the transformation,
// and no guess works for every field here — "FeedURL" lowercased is "feedURL", not "feedUrl".
func jsonFieldName(req any, goField string) string {
	t := reflect.TypeOf(req)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return goField
	}

	field, ok := t.FieldByName(goField)
	if !ok {
		return goField
	}

	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return goField
	}

	name, _, _ := strings.Cut(tag, ",")
	if name == "" || name == "-" {
		return goField
	}
	return name
}

// parseSourceID reads the :id path parameter, answering 400 when it is not an ObjectID.
func parseSourceID(c *gin.Context) (bson.ObjectID, bool) {
	id, err := bson.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid source id", nil)
		return bson.ObjectID{}, false
	}
	return id, true
}

// respondSourceError maps a usecase error to the HTTP status it deserves.
func (h *SourceHandler) respondSourceError(c *gin.Context, err error) {
	var configErr *domain.SourceConfigError
	switch {
	case errors.As(err, &configErr):
		RespondError(c, http.StatusBadRequest, "BAD_REQUEST", "Bad Request", []ErrorDetail{
			{Field: configErr.Field, Message: configErr.Reason},
		})
	case errors.Is(err, domain.ErrSourceNotFound):
		RespondError(c, http.StatusNotFound, "NOT_FOUND", "source not found", nil)
	case errors.Is(err, domain.ErrSourceExists):
		RespondError(c, http.StatusConflict, "CONFLICT", domain.ErrSourceExists.Error(), nil)
	default:
		RespondInternalError(c)
	}
}
