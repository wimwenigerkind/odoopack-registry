package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/worker"
)

type WebhookHandler struct {
	addons *repository.AddonRepository
	tokens middleware.TokenStore
	queue  *worker.Queue
}

// maxWebhookBytes bounds the push payload. Only repository.full_name is read,
// and a push event with its commit list stays far below 1 MiB.
const maxWebhookBytes = 1 << 20

func NewWebhookHandler(addons *repository.AddonRepository, tokens middleware.TokenStore, queue *worker.Queue) *WebhookHandler {
	return &WebhookHandler{addons: addons, tokens: tokens, queue: queue}
}

type forgejoPushPayload struct {
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (h *WebhookHandler) Forgejo(c *gin.Context) {
	ownerID, ok := middleware.BearerUserID(c, h.tokens, time.Now())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing api token"})
		return
	}

	event := c.GetHeader("X-Forgejo-Event")
	if event == "" {
		event = c.GetHeader("X-Gitea-Event")
	}
	if event != "push" {
		c.JSON(http.StatusAccepted, gin.H{"status": "ignored", "event": event})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBytes))
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload too large"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read body"})
		return
	}

	var payload forgejoPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json payload"})
		return
	}

	repoPath := payload.Repository.FullName
	if repoPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing repository.full_name"})
		return
	}

	addons, err := h.addons.ListByOwnerAndRepoPath(ownerID, repoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	queued := h.enqueueSyncs(addons)
	slog.Info("forgejo webhook", "owner", ownerID, "repo", repoPath, "queued", queued)
	c.JSON(http.StatusOK, gin.H{"queued": queued})
}

func (h *WebhookHandler) enqueueSyncs(addons []models.Addon) int {
	queued := 0
	for _, addon := range addons {
		if addon.Repo == nil {
			continue
		}
		if err := h.queue.Enqueue(worker.SyncJob{
			AddonID:       addon.ID,
			Name:          addon.Name,
			GitURL:        addon.Repo.GitURL,
			DefaultBranch: addon.Repo.DefaultBranch,
			Subpath:       addon.Subpath,
			Trigger:       "webhook",
			IntegrationID: addon.Repo.IntegrationID,
		}); err != nil {
			slog.Error("enqueue webhook sync", "addon", addon.ID, "err", err)
			continue
		}
		queued++
	}
	return queued
}
