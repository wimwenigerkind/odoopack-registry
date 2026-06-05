package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

const (
	contextKeyUserID  = "auth.user_id"
	contextKeySession = "auth.session"
)

func RequireAdmin(sessions *auth.SessionStore, users *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok := loadSession(c, sessions)
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

func RequireAuth(sessions *auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		sess, ok := loadSession(c, sessions)
		if !ok || sess.UserID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		c.Set(contextKeyUserID, sess.UserID)
		c.Set(contextKeySession, sess)
		c.Next()
	}
}

func OptionalAuth(sessions *auth.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sess, ok := loadSession(c, sessions); ok && sess.UserID != uuid.Nil {
			c.Set(contextKeyUserID, sess.UserID)
			c.Set(contextKeySession, sess)
		}
		c.Next()
	}
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

func loadSession(c *gin.Context, sessions *auth.SessionStore) (auth.Session, bool) {
	cookie, err := c.Cookie(auth.SessionCookieName)
	if err != nil || cookie == "" {
		return auth.Session{}, false
	}
	sess, err := sessions.Get(cookie)
	if err != nil {
		return auth.Session{}, false
	}
	return sess, true
}
