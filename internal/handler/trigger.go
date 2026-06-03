package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/worker"
)

type TriggerHandler struct {
	addons *repository.AddonRepository
	queue  *worker.Queue
}

func NewTriggerHandler(addons *repository.AddonRepository, queue *worker.Queue) *TriggerHandler {
	return &TriggerHandler{addons: addons, queue: queue}
}

func (h *TriggerHandler) Sync(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
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
	if !canWriteAddon(c, addon) {
		c.JSON(http.StatusNotFound, gin.H{"error": "addon not found"})
		return
	}

	h.queue.Enqueue(worker.SyncJob{
		AddonID:       addon.ID,
		Name:          addon.Name,
		GitURL:        addon.GitURL,
		DefaultBranch: addon.DefaultBranch,
		Trigger:       "manual",
	})

	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}
