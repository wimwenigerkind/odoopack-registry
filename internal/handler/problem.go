package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// The ErrorCode values of docs/openapi.yaml; both lists change together.
const (
	codeMalformedRequest = "malformed_request"
	codeNotFound         = "not_found"
	codeConflict         = "conflict"
	codeInternalError    = "internal_error"
)

// problemDetails is the Problem schema of docs/openapi.yaml (RFC 9457). The
// type stays about:blank because no page documents the codes yet, so title is
// the status phrase as the RFC asks and code carries the meaning.
type problemDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail,omitempty"`
	Code   string `json:"code"`
}

// problem answers with application/problem+json. Only the /registry/v1 routes
// use it so far; the /api/v1 routes of the frontend still send {"error": ...}.
func problem(c *gin.Context, status int, code, detail string) {
	// gin keeps a Content-Type that is already set instead of its JSON default.
	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, problemDetails{
		Type:   "about:blank",
		Title:  http.StatusText(status),
		Status: status,
		Detail: detail,
		Code:   code,
	})
}
