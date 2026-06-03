package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wimwenigerkind/odoopack-registry/internal/auth"
	"github.com/wimwenigerkind/odoopack-registry/internal/middleware"
	"github.com/wimwenigerkind/odoopack-registry/internal/models"
	"github.com/wimwenigerkind/odoopack-registry/internal/repository"
)

const (
	stateCookieName   = "oauth_state"
	loginTTL          = 10 * time.Minute
	sessionCookiePath = "/"
	sessionTTL        = 30 * 24 * time.Hour
)

var (
	errRegistrationDisabled = errors.New("registration disabled")
	errEmailNotVerified     = errors.New("email not verified")
)

type AuthHandler struct {
	registry     *auth.Registry
	stateStore   *auth.StateStore
	sessions     *auth.SessionStore
	users        *repository.UserRepository
	cookieSecure bool
}

func NewAuthHandler(reg *auth.Registry, store *auth.StateStore, sessions *auth.SessionStore, users *repository.UserRepository, cookieSecure bool) *AuthHandler {
	return &AuthHandler{
		registry:     reg,
		stateStore:   store,
		sessions:     sessions,
		users:        users,
		cookieSecure: cookieSecure,
	}
}

func statePathFor(provider string) string {
	return "/auth/" + provider
}

func (h *AuthHandler) writeCookie(c *gin.Context, name, value, path string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(name, value, maxAge, path, "", h.cookieSecure, true)
}

func internalError(c *gin.Context, op string, err error) {
	log.Printf("auth %s: %v", op, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

func (h *AuthHandler) Login(c *gin.Context) {
	providerName := c.Param("provider")
	provider, err := h.registry.Get(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider"})
		return
	}
	if !provider.AllowLogin() {
		c.JSON(http.StatusForbidden, gin.H{"error": "login disabled for provider"})
		return
	}

	state := auth.NewState()
	nonce := auth.NewState()
	verifier := auth.NewVerifier()

	if err := h.stateStore.Save(state, auth.FlowState{
		Provider: provider.Name(),
		Nonce:    nonce,
		Verifier: verifier,
		ReturnTo: sanitizeReturnTo(c.Query("return_to")),
	}, loginTTL); err != nil {
		internalError(c, "save state", err)
		return
	}

	h.writeCookie(c, stateCookieName, state, statePathFor(provider.Name()), int(loginTTL.Seconds()))
	c.Redirect(http.StatusFound, provider.AuthURL(state, nonce, verifier))
}

func (h *AuthHandler) Callback(c *gin.Context) {
	providerName := c.Param("provider")
	statePath := statePathFor(providerName)

	cookieState, err := c.Cookie(stateCookieName)
	if err != nil || cookieState == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing state cookie"})
		return
	}
	urlState := c.Query("state")
	if urlState == "" || cookieState != urlState {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state mismatch"})
		return
	}
	h.writeCookie(c, stateCookieName, "", statePath, -1)

	if errParam := c.Query("error"); errParam != "" {
		log.Printf("auth callback: provider %q returned error: %s (%s)", providerName, errParam, c.Query("error_description"))
		c.JSON(http.StatusBadRequest, gin.H{"error": "authentication failed at provider"})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}

	fs, err := h.stateStore.Take(urlState)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or expired state"})
		return
	}
	if fs.Provider != providerName {
		c.JSON(http.StatusBadRequest, gin.H{"error": "provider mismatch"})
		return
	}

	provider, err := h.registry.Get(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider"})
		return
	}

	subject, email, emailVerified, err := provider.Exchange(c.Request.Context(), code, fs.Verifier, fs.Nonce)
	if err != nil {
		log.Printf("auth callback: exchange %q: %v", providerName, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return
	}

	user, err := h.findOrCreateUser(provider, subject, email, emailVerified)
	if err != nil {
		switch {
		case errors.Is(err, errRegistrationDisabled):
			c.JSON(http.StatusForbidden, gin.H{"error": "registration via this provider is disabled"})
		case errors.Is(err, errEmailNotVerified):
			c.JSON(http.StatusForbidden, gin.H{"error": "email not verified by provider"})
		default:
			internalError(c, "find/create user", err)
		}
		return
	}

	if oldCookie, err := c.Cookie(auth.SessionCookieName); err == nil && oldCookie != "" {
		_ = h.sessions.Delete(oldCookie)
	}

	sess, err := h.sessions.Create(user.ID, sessionTTL, c.GetHeader("User-Agent"), c.ClientIP())
	if err != nil {
		internalError(c, "create session", err)
		return
	}
	h.writeCookie(c, auth.SessionCookieName, sess.ID, sessionCookiePath, int(sessionTTL.Seconds()))

	returnTo := fs.ReturnTo
	if returnTo == "" {
		returnTo = "/"
	}
	c.Redirect(http.StatusFound, returnTo)
}

// FIXME: username already exists
func deriveUsername(email string) string {
	at := strings.Index(email, "@")
	if at <= 0 {
		return ""
	}
	return email[:at]
}

func sanitizeReturnTo(p string) string {
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	if strings.HasPrefix(p, "//") {
		return ""
	}
	if strings.Contains(p, "\\") || strings.Contains(p, "\n") || strings.Contains(p, "\r") {
		return ""
	}
	return p
}

func (h *AuthHandler) findOrCreateUser(provider auth.Provider, subject, email string, emailVerified bool) (*models.User, error) {
	user, err := h.users.GetByIdentity(provider.Name(), subject)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if !provider.AllowRegister() {
		return nil, errRegistrationDisabled
	}
	if email == "" || !emailVerified {
		return nil, errEmailNotVerified
	}

	newUser := &models.User{
		Email:    email,
		Username: deriveUsername(email),
	}
	newIdentity := &models.Identity{
		Subject:  subject,
		Provider: provider.Name(),
	}
	if err := h.users.CreateWithIdentity(newUser, newIdentity); err != nil {
		if u, lookupErr := h.users.GetByIdentity(provider.Name(), subject); lookupErr == nil {
			return u, nil
		}
		return nil, err
	}
	return newUser, nil
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	user, err := h.users.GetByID(userID)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err != nil {
		internalError(c, "get user", err)
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if cookie, err := c.Cookie(auth.SessionCookieName); err == nil && cookie != "" {
		_ = h.sessions.Delete(cookie)
	}
	h.writeCookie(c, auth.SessionCookieName, "", sessionCookiePath, -1)
	c.JSON(http.StatusOK, gin.H{"status": "logged out"})
}

type providerSummary struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (h *AuthHandler) ListProviders(c *gin.Context) {
	providers := h.registry.EnabledForLogin()
	out := make([]providerSummary, 0, len(providers))
	for _, p := range providers {
		out = append(out, providerSummary{
			Name: p.Name(),
			Type: string(p.Type()),
		})
	}
	c.JSON(http.StatusOK, gin.H{"providers": out})
}
