package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

type UserHandler struct {
	users *repository.UserRepository
}

func NewUserHandler(users *repository.UserRepository) *UserHandler {
	return &UserHandler{users: users}
}

func (h *UserHandler) List(c *gin.Context) {
	users, err := h.users.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, users)
}
