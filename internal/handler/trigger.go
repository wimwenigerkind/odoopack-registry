package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/worker"
)

type TriggerHandler struct {
	addons   *repository.AddonRepository
	users    *repository.UserRepository
	triggers *repository.SyncTriggerRepository
	queue    *worker.Queue
	limiter  *userLimiter
	interval time.Duration
}

func NewTriggerHandler(addons *repository.AddonRepository, users *repository.UserRepository, triggers *repository.SyncTriggerRepository, queue *worker.Queue, interval time.Duration) *TriggerHandler {
	return &TriggerHandler{
		addons:   addons,
		users:    users,
		triggers: triggers,
		queue:    queue,
		limiter:  newUserLimiter(syncBurst, syncRefill, time.Now),
		interval: interval,
	}
}

func (h *TriggerHandler) Sync(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if userID, ok := middleware.CurrentUserID(c); ok {
		if wait, ok := h.limiter.allow(userID); !ok {
			tooManyRequests(c, wait)
			return
		}
	}

	addon, err := h.addons.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if !canWriteAddon(c, addon, h.users) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}

	if addon.Repo == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "addon has no repo"})
		return
	}
	if h.interval > 0 {
		wait, err := h.triggers.Claim(addon.ID, h.interval)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		if wait > 0 {
			tooManyRequests(c, wait)
			return
		}
	}
	if err := h.queue.Enqueue(worker.SyncJob{
		AddonID:       addon.ID,
		Name:          addon.Name,
		GitURL:        addon.Repo.GitURL,
		DefaultBranch: addon.Repo.DefaultBranch,
		Subpath:       addon.Subpath,
		Trigger:       "manual",
		IntegrationID: addon.Repo.IntegrationID,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not queue sync"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}
