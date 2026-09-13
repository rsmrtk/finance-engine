package rest

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	authsvc "github.com/rsmrtk/finance-engine/internal/service/auth"
	sessionsvc "github.com/rsmrtk/finance-engine/internal/service/session"
)

const (
	accessCookieName  = "access_token"
	refreshCookieName = "refresh_token"
)

type authHandler struct {
	auth          *authsvc.Service
	sessions      *sessionsvc.Service
	corsOrigin    string
	accessMaxAge  time.Duration
	refreshMaxAge time.Duration
}

func newAuthHandler(auth *authsvc.Service, sessions *sessionsvc.Service, corsOrigin string, accessMaxAge, refreshMaxAge time.Duration) *authHandler {
	return &authHandler{auth: auth, sessions: sessions, corsOrigin: corsOrigin, accessMaxAge: accessMaxAge, refreshMaxAge: refreshMaxAge}
}

type emailPasswordRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type googleRequest struct {
	IDToken string `json:"id_token"`
}

func (h *authHandler) signup(w http.ResponseWriter, r *http.Request) {
	var req emailPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.auth.SignUpWithEmail(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	h.issueAndRespond(w, r, result)
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var req emailPasswordRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.auth.SignInWithEmail(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	h.issueAndRespond(w, r, result)
}

func (h *authHandler) google(w http.ResponseWriter, r *http.Request) {
	var req googleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.auth.SignInWithGoogle(r.Context(), req.IDToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	h.issueAndRespond(w, r, result)
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	user, err := h.auth.Me(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToJSON(user)})
}

type updatePreferencesRequest struct {
	Theme         string `json:"theme"`
	GradientColor string `json:"gradientColor"`
}

func (h *authHandler) updatePreferences(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req updatePreferencesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.auth.UpdatePreferences(r.Context(), userID, req.Theme, req.GradientColor)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToJSON(user)})
}

type updateProfileRequest struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

func (h *authHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req updateProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.auth.UpdateProfile(r.Context(), userID, req.Name, req.Avatar)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToJSON(user)})
}

type updateGoalsRequest struct {
	Goals string `json:"goals"`
}

func (h *authHandler) updateGoals(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req updateGoalsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.auth.UpdateGoals(r.Context(), userID, req.Goals)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToJSON(user)})
}

type updateCurrencyRequest struct {
	Currency string `json:"currency"`
}

func (h *authHandler) updateCurrency(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	var req updateCurrencyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	user, err := h.auth.UpdateBaseCurrency(r.Context(), userID, req.Currency)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": userToJSON(user)})
}

func (h *authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "missing refresh token")
		return
	}
	pair, err := h.sessions.Refresh(r.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, sessionsvc.ErrInvalidRefreshToken) {
			h.clearCookies(w)
			writeError(w, http.StatusUnauthorized, "session expired, please log in again")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to refresh session")
		return
	}
	h.setCookies(w, pair)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *authHandler) listSessions(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	rawRefresh := ""
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		rawRefresh = cookie.Value
	}
	sessions, err := h.sessions.ListActive(r.Context(), userID, rawRefresh)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load sessions")
		return
	}
	out := make([]sessionJSON, len(sessions))
	for i, s := range sessions {
		out[i] = sessionToJSON(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": out})
}

func (h *authHandler) revokeSession(w http.ResponseWriter, r *http.Request) {
	userID, _ := userIDFromContext(r.Context())
	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}
	if err := h.sessions.RevokeForUser(r.Context(), userID, sessionID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookieName); err == nil {
		_ = h.sessions.Revoke(r.Context(), cookie.Value)
	}
	h.clearCookies(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *authHandler) issueAndRespond(w http.ResponseWriter, r *http.Request, result authsvc.SignInResult) {
	pair, err := h.sessions.Issue(r.Context(), result.User.ID, r.Header.Get("User-Agent"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	h.setCookies(w, pair)
	writeJSON(w, http.StatusOK, map[string]any{
		"user": userToJSON(result.User),
	})
}

// secureCookies mirrors the frontend origin's scheme: Secure cookies are
// required for SameSite=None in production (https), but must be left off
// for local http dev, where the browser would otherwise silently refuse
// to store them.
func (h *authHandler) secureCookies() bool {
	return strings.HasPrefix(h.corsOrigin, "https://")
}

func (h *authHandler) setCookies(w http.ResponseWriter, pair sessionsvc.Pair) {
	http.SetCookie(w, &http.Cookie{
		Name: accessCookieName, Value: pair.AccessToken, Path: "/",
		HttpOnly: true, Secure: h.secureCookies(), SameSite: http.SameSiteLaxMode,
		MaxAge: int(h.accessMaxAge.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: pair.RefreshToken, Path: "/api/auth",
		HttpOnly: true, Secure: h.secureCookies(), SameSite: http.SameSiteLaxMode,
		MaxAge: int(h.refreshMaxAge.Seconds()),
	})
}

func (h *authHandler) clearCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: accessCookieName, Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: "", Path: "/api/auth", MaxAge: -1})
}
