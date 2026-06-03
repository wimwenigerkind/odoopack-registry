package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
)

func currentUserIDPtr(c *gin.Context) *uuid.UUID {
	if id, ok := middleware.CurrentUserID(c); ok {
		return &id
	}
	return nil
}

func canReadAddon(c *gin.Context, addon *models.Addon) bool {
	if addon.Visibility == models.VisibilityPublic {
		return true
	}
	uid, ok := middleware.CurrentUserID(c)
	if !ok {
		return false
	}
	return addon.OwnerID == uid
}

func canWriteAddon(c *gin.Context, addon *models.Addon) bool {
	uid, ok := middleware.CurrentUserID(c)
	if !ok {
		return false
	}
	return addon.OwnerID == uid
}
