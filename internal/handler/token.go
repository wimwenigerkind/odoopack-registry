package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

type TokenHandler struct {
	tokens *repository.ApiTokenRepository
}

func NewTokenHandler(tokens *repository.ApiTokenRepository) *TokenHandler {
	return &TokenHandler{tokens: tokens}
}

// createTokenRequest takes an optional lifetime in whole days; zero or absent
// means the token never expires.
type createTokenRequest struct {
	Name          string `json:"name" binding:"required"`
	ExpiresInDays int    `json:"expires_in_days" binding:"omitempty,min=1,max=3650"`
}

// createTokenResponse is the only response that carries the plain token.
type createTokenResponse struct {
	models.ApiToken
	Token string `json:"token"`
}

func newApiToken(userID uuid.UUID, req createTokenRequest, plain string, now time.Time) models.ApiToken {
	prefix, lastFour := apitoken.DisplayParts(plain)
	tok := models.ApiToken{
		UserID:    userID,
		Name:      req.Name,
		TokenHash: apitoken.Hash(plain),
		Prefix:    prefix,
		LastFour:  lastFour,
	}
	if req.ExpiresInDays > 0 {
		expiresAt := now.AddDate(0, 0, req.ExpiresInDays)
		tok.ExpiresAt = &expiresAt
	}
	return tok
}

func (h *TokenHandler) Create(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req createTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plain, err := apitoken.Generate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	tok := newApiToken(userID, req, plain, time.Now())
	if err := h.tokens.Create(&tok); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusCreated, createTokenResponse{ApiToken: tok, Token: plain})
}

func (h *TokenHandler) List(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	tokens, err := h.tokens.ListByUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, tokens)
}

func (h *TokenHandler) Delete(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.tokens.Delete(id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.Status(http.StatusNoContent)
}
