package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

func currentUserIDPtr(c *gin.Context) *uuid.UUID {
	if id, ok := middleware.CurrentUserID(c); ok {
		return &id
	}
	return nil
}

func isCurrentUserAdmin(c *gin.Context, users *repository.UserRepository) bool {
	uid, ok := middleware.CurrentUserID(c)
	if !ok {
		return false
	}
	u, err := users.GetByID(uid)
	if err != nil || u == nil {
		return false
	}
	return u.IsAdmin
}

func canReadAddon(c *gin.Context, addon *models.Addon, groups *repository.GroupRepository, users *repository.UserRepository, mode string) bool {
	if addon.Visibility == models.VisibilityPublic && mode != "private" {
		return true
	}
	uid, ok := middleware.CurrentUserID(c)
	if !ok {
		return false
	}
	if addon.Visibility == models.VisibilityPublic {
		return true
	}
	if isCurrentUserAdmin(c, users) {
		return true
	}
	if addon.OwnerID == uid {
		return true
	}
	can, err := groups.UserCanReadAddon(uid, addon.ID)
	if err != nil {
		return false
	}
	return can
}

func canWriteAddon(c *gin.Context, addon *models.Addon, users *repository.UserRepository) bool {
	uid, ok := middleware.CurrentUserID(c)
	if !ok {
		return false
	}
	if addon.OwnerID == uid {
		return true
	}
	return isCurrentUserAdmin(c, users)
}
