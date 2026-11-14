package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/apitoken"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

const (
	contextKeyUserID  = "auth.user_id"
	contextKeySession = "auth.session"
)

func RequireAdmin(sessions auth.SessionStore, users *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok, err := loadSession(c, sessions)
		if err != nil {
			abortStoreUnavailable(c)
			return
		}
		if !ok || sess.UserID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		user, err := users.GetByID(sess.UserID)
		if err != nil || user == nil || !user.IsAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
			return
		}
		c.Set(contextKeyUserID, sess.UserID)
		c.Set(contextKeySession, sess)
		c.Next()
	}
}

func RequireAuth(sessions auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok, err := loadSession(c, sessions)
		if err != nil {
			abortStoreUnavailable(c)
			return
		}
		if !ok || sess.UserID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		c.Set(contextKeyUserID, sess.UserID)
		c.Set(contextKeySession, sess)
		c.Next()
	}
}

func OptionalAuth(sessions auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok, err := loadSession(c, sessions)
		if err != nil {
			// The store logs the outage once; the caller continues as anonymous.
			slog.Debug("session store unavailable, continuing anonymously", "err", err)
		}
		if ok && sess.UserID != uuid.Nil {
			c.Set(contextKeyUserID, sess.UserID)
			c.Set(contextKeySession, sess)
		}
		c.Next()
	}
}

// TokenStore is the part of the token repository that bearer authentication needs.
type TokenStore interface {
	GetByHash(hash string) (*models.ApiToken, error)
	TouchLastUsed(id uuid.UUID, now time.Time) error
}

func ApiKeyOptional(tokens TokenStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, exists := c.Get(contextKeyUserID); exists {
			c.Next()
			return
		}
		if userID, ok := BearerUserID(c, tokens, time.Now()); ok {
			c.Set(contextKeyUserID, userID)
		}
		c.Next()
	}
}

func BearerUserID(c *gin.Context, tokens TokenStore, now time.Time) (uuid.UUID, bool) {
	if tokens == nil {
		return uuid.Nil, false
	}
	header := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return uuid.Nil, false
	}
	plain := strings.TrimSpace(header[len(prefix):])
	if plain == "" {
		return uuid.Nil, false
	}
	tok, err := tokens.GetByHash(apitoken.Hash(plain))
	if err != nil || tok == nil || apitoken.Expired(tok.ExpiresAt, now) {
		return uuid.Nil, false
	}
	_ = tokens.TouchLastUsed(tok.ID, now)
	return tok.UserID, true
}

func CurrentUserID(c *gin.Context) (uuid.UUID, bool) {
	raw, ok := c.Get(contextKeyUserID)
	if !ok {
		return uuid.Nil, false
	}
	id, ok := raw.(uuid.UUID)
	return id, ok
}

func CurrentSession(c *gin.Context) (auth.Session, bool) {
	raw, ok := c.Get(contextKeySession)
	if !ok {
		return auth.Session{}, false
	}
	sess, ok := raw.(auth.Session)
	return sess, ok
}

// loadSession returns an error only when the store cannot be reached; a
// missing, unknown or expired session is not ok and no error.
func loadSession(c *gin.Context, sessions auth.SessionStore) (auth.Session, bool, error) {
	cookie, err := c.Cookie(auth.SessionCookieName)
	if err != nil || cookie == "" {
		return auth.Session{}, false, nil
	}
	sess, err := sessions.Get(cookie)
	if errors.Is(err, auth.ErrStoreUnavailable) {
		return auth.Session{}, false, err
	}
	if err != nil {
		return auth.Session{}, false, nil
	}
	return sess, true, nil
}

// abortStoreUnavailable answers 503 so a client can tell a Redis outage from
// a logged out session and retry instead of sending the user to log in.
func abortStoreUnavailable(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "session store unavailable"})
}
