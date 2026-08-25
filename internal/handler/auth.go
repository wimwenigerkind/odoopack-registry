package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	stateStore   auth.StateStore
	sessions     auth.SessionStore
	users        *repository.UserRepository
	integrations *repository.IntegrationRepository
	cookieSecure bool
}

func NewAuthHandler(reg *auth.Registry, store auth.StateStore, sessions auth.SessionStore, users *repository.UserRepository, integrations *repository.IntegrationRepository, cookieSecure bool) *AuthHandler {
	return &AuthHandler{
		registry:     reg,
		stateStore:   store,
		sessions:     sessions,
		users:        users,
		integrations: integrations,
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
	slog.Error("auth", "op", op, "err", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

func (h *AuthHandler) Link(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	providerName := c.Param("provider")
	provider, err := h.registry.GetLogin(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider"})
		return
	}

	state := auth.NewState()
	nonce := auth.NewState()
	verifier := auth.NewVerifier()

	if err := h.stateStore.Save(state, auth.FlowState{
		Provider: provider.Name(),
		Kind:     auth.FlowLink,
		Nonce:    nonce,
		Verifier: verifier,
		ReturnTo: sanitizeReturnTo(c.Query("return_to")),
		UserID:   userID.String(),
	}, loginTTL); err != nil {
		internalError(c, "save state", err)
		return
	}

	h.writeCookie(c, stateCookieName, state, statePathFor(provider.Name()), int(loginTTL.Seconds()))
	c.Redirect(http.StatusFound, provider.LoginAuthURL(state, nonce, verifier))
}

func (h *AuthHandler) Login(c *gin.Context) {
	providerName := c.Param("provider")
	provider, err := h.registry.GetLogin(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider"})
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
	c.Redirect(http.StatusFound, provider.LoginAuthURL(state, nonce, verifier))
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
		slog.Warn("auth callback: provider error", "provider", providerName, "error", errParam, "description", c.Query("error_description"))
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

	if fs.Kind == auth.FlowIntegration {
		h.completeIntegration(c, providerName, code, fs)
		return
	}
	if fs.Kind == auth.FlowLink {
		h.completeLink(c, providerName, code, fs)
		return
	}

	provider, err := h.registry.GetLogin(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider"})
		return
	}

	subject, email, emailVerified, err := provider.ExchangeLogin(c.Request.Context(), code, fs.Verifier, fs.Nonce)
	if err != nil {
		slog.Warn("auth callback: exchange failed", "provider", providerName, "err", err)
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

func (h *AuthHandler) findOrCreateUser(provider auth.LoginProvider, subject, email string, emailVerified bool) (*models.User, error) {
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

func (h *AuthHandler) completeLink(c *gin.Context, providerName, code string, fs auth.FlowState) {
	provider, err := h.registry.GetLogin(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown provider"})
		return
	}
	userID, err := uuid.Parse(fs.UserID)
	if err != nil || userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user in state"})
		return
	}

	subject, _, _, err := provider.ExchangeLogin(c.Request.Context(), code, fs.Verifier, fs.Nonce)
	if err != nil {
		slog.Warn("link callback: exchange failed", "provider", providerName, "err", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication failed"})
		return
	}

	existing, err := h.users.GetByIdentity(providerName, subject)
	if err == nil && existing != nil {
		if existing.ID == userID {
			c.Redirect(http.StatusFound, linkReturnTo(fs))
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "account already linked to another user"})
		return
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		internalError(c, "lookup identity", err)
		return
	}

	identity := &models.Identity{Provider: providerName, Subject: subject}
	if err := h.users.AttachIdentity(userID, identity); err != nil {
		internalError(c, "attach identity", err)
		return
	}

	c.Redirect(http.StatusFound, linkReturnTo(fs))
}

func linkReturnTo(fs auth.FlowState) string {
	if fs.ReturnTo == "" {
		return "/profile"
	}
	return fs.ReturnTo
}

func (h *AuthHandler) UnlinkIdentity(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	err = h.users.DeleteIdentity(id, userID)
	if errors.Is(err, repository.ErrLastIdentity) {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot remove last identity"})
		return
	}
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}
	if err != nil {
		internalError(c, "delete identity", err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) completeIntegration(c *gin.Context, providerName, code string, fs auth.FlowState) {
	provider, err := h.registry.GetIntegration(providerName)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown integration provider"})
		return
	}
	userID, err := uuid.Parse(fs.UserID)
	if err != nil || userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user in state"})
		return
	}

	accessToken, refreshToken, expiresAt, err := provider.ExchangeIntegration(c.Request.Context(), code)
	if err != nil {
		slog.Warn("integration callback: exchange failed", "provider", providerName, "err", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "integration authorization failed"})
		return
	}

	accountName, accountErr := provider.FetchAccountName(c.Request.Context(), accessToken)
	if accountErr != nil {
		slog.Warn("integration callback: fetch account name", "provider", providerName, "err", accountErr)
	}

	hookSecret, err := generateSecret(32)
	if err != nil {
		internalError(c, "generate hook secret", err)
		return
	}

	it := &models.OAuthIntegration{
		Provider:     providerName,
		OwnerID:      userID,
		AccountName:  accountName,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		HookSecret:   hookSecret,
	}
	if err := h.integrations.Create(it); err != nil {
		internalError(c, "save integration", err)
		return
	}

	returnTo := fs.ReturnTo
	if returnTo == "" {
		returnTo = "/profile"
	}
	c.Redirect(http.StatusFound, returnTo)
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
	user.GravatarHash = hashEmailForGravatar(user.Email)
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
