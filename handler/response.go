package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/erivelto/read-tracker/tracker/domain"
)

// DataEnvelope wraps a successful response payload.
type DataEnvelope struct {
	Data interface{} `json:"data"`
}

// TitlesEnvelope wraps the titles list response.
type TitlesEnvelope struct {
	Titles []domain.Title `json:"titles"`
}

// ErrorEnvelope wraps an error response.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains error metadata returned to clients.
type ErrorBody struct {
	Code    string        `json:"code"`
	Message string        `json:"message"`
	Details []ErrorDetail `json:"details,omitempty"`
}

// ErrorDetail describes a single field-level validation failure.
type ErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// RespondData writes a JSON success response with the given status code.
func RespondData(c *gin.Context, status int, data interface{}) {
	c.JSON(status, DataEnvelope{Data: data})
}

// RespondTitles writes a JSON success response containing titles.
func RespondTitles(c *gin.Context, status int, titles []domain.Title) {
	c.JSON(status, TitlesEnvelope{Titles: titles})
}

// RespondError writes a JSON error response with the given status code and error details.
func RespondError(c *gin.Context, status int, code, message string, details []ErrorDetail) {
	c.JSON(status, ErrorEnvelope{Error: ErrorBody{
		Code: code,
		Message: message,
		Details: details,
	},
	})
}

// RespondInternalError writes a generic 500 response without exposing internal details.
func RespondInternalError(c *gin.Context) {
	RespondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "an unexpected error occurred", nil)
}
