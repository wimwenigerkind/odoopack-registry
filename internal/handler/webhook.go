package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
	"github.com/wimwenigerkind/odoopack-registry/internal/worker"
)

type WebhookHandler struct {
	integrations *repository.IntegrationRepository
	addons       *repository.AddonRepository
	queue        *worker.Queue
}

func NewWebhookHandler(integrations *repository.IntegrationRepository, addons *repository.AddonRepository, queue *worker.Queue) *WebhookHandler {
	return &WebhookHandler{integrations: integrations, addons: addons, queue: queue}
}

type bitbucketPushPayload struct {
	Repository struct {
		FullName string `json:"full_name"`
		UUID     string `json:"uuid"`
	} `json:"repository"`
	Push struct {
		Changes []struct {
			New *struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"new"`
		} `json:"changes"`
	} `json:"push"`
}

func (h *WebhookHandler) Bitbucket(c *gin.Context) {
	id, err := uuid.Parse(c.Param("integration_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid integration id"})
		return
	}

	it, err := h.integrations.GetByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "integration not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if it.Provider != "bitbucket" {
		c.JSON(http.StatusConflict, gin.H{"error": "integration is not a bitbucket integration"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read body"})
		return
	}

	sigHeader := c.GetHeader("X-Hub-Signature")
	if !verifyBitbucketHMAC(it.HookSecret, body, sigHeader) {
		slog.Warn("webhook signature mismatch",
			"integration", it.ID,
			"secret_present", it.HookSecret != "",
			"sig_header", sigHeader,
			"body_len", len(body),
		)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	event := c.GetHeader("X-Event-Key")
	if event != "repo:push" {
		c.JSON(http.StatusAccepted, gin.H{"status": "ignored", "event": event})
		return
	}

	var payload bitbucketPushPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json payload"})
		return
	}

	repoPath := payload.Repository.FullName
	if repoPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing repository.full_name"})
		return
	}

	addons, err := h.addons.ListByOwnerAndRepoPath(it.OwnerID, repoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	queued := 0
	for _, addon := range addons {
		h.queue.Enqueue(worker.SyncJob{
			AddonID:       addon.ID,
			Name:          addon.Name,
			GitURL:        addon.GitURL,
			DefaultBranch: addon.DefaultBranch,
			Trigger:       "webhook",
			IntegrationID: addon.IntegrationID,
		})
		queued++
	}
	slog.Info("bitbucket webhook", "integration", it.ID, "repo", repoPath, "queued", queued)
	c.JSON(http.StatusOK, gin.H{"queued": queued})
}

func verifyBitbucketHMAC(secret string, body []byte, header string) bool {
	if secret == "" || header == "" {
		return false
	}
	const prefix = "sha256="
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	sigHex := strings.TrimPrefix(header, prefix)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHex))
}
