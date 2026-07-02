package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

func hashEmailForGravatar(email string) string {
	n := strings.ToLower(strings.TrimSpace(email))
	if n == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(n))
	return hex.EncodeToString(sum[:])
}

func sanitizeOwner(u *models.User) {
	if u == nil {
		return
	}
	u.GravatarHash = hashEmailForGravatar(u.Email)
	u.Email = ""
}

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
	if addon.Repo != nil && addon.Repo.OwnerID == uid {
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
	if addon.Repo != nil && addon.Repo.OwnerID == uid {
		return true
	}
	return isCurrentUserAdmin(c, users)
}

func canReadRepo(c *gin.Context, repo *models.Repo, addons []models.Addon, groups *repository.GroupRepository, users *repository.UserRepository, mode string) bool {
	uid, ok := middleware.CurrentUserID(c)
	if ok && repo.OwnerID == uid {
		return true
	}
	if isCurrentUserAdmin(c, users) {
		return true
	}
	for i := range addons {
		addon := addons[i]
		addon.Repo = repo
		if canReadAddon(c, &addon, groups, users, mode) {
			return true
		}
	}
	return false
}

func canWriteRepo(c *gin.Context, repo *models.Repo, users *repository.UserRepository) bool {
	uid, ok := middleware.CurrentUserID(c)
	if !ok {
		return false
	}
	if repo.OwnerID == uid {
		return true
	}
	return isCurrentUserAdmin(c, users)
}
