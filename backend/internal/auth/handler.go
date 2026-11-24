package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"uptime-app/backend/internal/httpx"
	"uptime-app/backend/internal/store"
)

const cookieName = "refresh_token"

// CredentialsRequest supplies the email and password for registration or login.
type CredentialsRequest struct {
	Email    string `json:"email" binding:"required" maxLength:"254" example:"user@example.com"`
	Password string `json:"password" binding:"required" minLength:"8" maxLength:"128" example:"correct horse battery staple"`
}

// SessionRequest is the empty JSON object required by refresh and logout.
type SessionRequest struct{}

// ProfileRequest supplies the name to trim and validate as 1–100 Unicode code points.
type ProfileRequest struct {
	// Name is validated after trimming: 1-100 Unicode code points without controls.
	Name string `json:"name" binding:"required" minLength:"1" example:"Vitaliy"`
}

// Handler owns auth HTTP input, responses, cookies and service error mapping.
type Handler struct {
	service *Service
	secure  bool
	now     func() time.Time
	avatars *AvatarStorage
}

// NewHandler configures refresh-cookie security and optional avatar storage.
func NewHandler(service *Service, secureCookies bool, avatars *AvatarStorage) *Handler {
	now := time.Now
	if service.Now != nil {
		now = service.Now
	}
	return &Handler{service: service, secure: secureCookies, now: now, avatars: avatars}
}

// Register accepts JSON credentials and returns 201 with public auth data and an HttpOnly refresh cookie.
//
// @Summary Register an account
// @Tags auth
// @Description Sets an HttpOnly refresh_token cookie scoped to /api/auth. Email is trimmed and lowercased; password is not normalized and must contain 8-128 Unicode code points.
// @Accept json
// @Produce json
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param credentials body CredentialsRequest true "Email and password; no other fields"
// @Success 201 {object} Result
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 409 {object} httpx.ErrorResponse "Email already registered"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/auth/register [post]
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	h.credentials(w, r, h.service.Register, http.StatusCreated)
}

// Login accepts JSON credentials and sets a refresh cookie for the new session.
//
// @Summary Sign in
// @Tags auth
// @Description Sets an HttpOnly refresh_token cookie scoped to /api/auth. Email is trimmed and lowercased; password is not normalized and must contain 8-128 Unicode code points.
// @Accept json
// @Produce json
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param credentials body CredentialsRequest true "Email and password; no other fields"
// @Success 200 {object} Result
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/auth/login [post]
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	h.credentials(w, r, h.service.Login, http.StatusOK)
}

func (h *Handler) credentials(w http.ResponseWriter, r *http.Request, action func(context.Context, string, string) (Result, error), status int) {
	var input CredentialsRequest
	if err := httpx.Decode(w, r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Expected a JSON object with email and password")
		return
	}
	result, err := action(r.Context(), input.Email, input.Password)
	if err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, result.Refresh, result.ExpiresAt)
	httpx.WriteJSON(w, status, result)
}

// Refresh rotates the HttpOnly refresh cookie and returns a new access JWT.
// Unauthorized refreshes clear the cookie; the JSON body must be an empty object.
//
// @Summary Rotate the refresh token
// @Tags auth
// @Description Requires the refresh_token HttpOnly cookie. Rotates it without extending the fixed 24-hour session. Reuse of an old token revokes the session.
// @Accept json
// @Produce json
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param body body SessionRequest true "Empty JSON object {}"
// @Success 200 {object} Result
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/auth/refresh [post]
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	raw, ok := h.sessionToken(w, r)
	if !ok {
		return
	}
	result, err := h.service.Refresh(r.Context(), raw)
	if err != nil {
		if errors.Is(err, store.ErrUnauthorized) {
			h.setCookie(w, "", time.Time{})
		}
		h.fail(w, err)
		return
	}
	h.setCookie(w, result.Refresh, result.ExpiresAt)
	httpx.WriteJSON(w, http.StatusOK, result)
}

// Logout accepts an empty JSON object, revokes the cookie's session and clears the cookie.
//
// @Summary Sign out
// @Tags auth
// @Description Uses the refresh_token cookie when present; missing or invalid cookies are accepted. Clears the cookie. Existing access JWTs remain valid until expiry.
// @Accept json
// @Produce json
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param body body SessionRequest true "Empty JSON object {}"
// @Success 204 "Session revoked and refresh cookie cleared"
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/auth/logout [post]
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	raw, ok := h.sessionToken(w, r)
	if !ok {
		return
	}
	if err := h.service.Logout(r.Context(), raw); err != nil {
		h.fail(w, err)
		return
	}
	h.setCookie(w, "", time.Time{})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) sessionToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	if err := httpx.Decode(w, r, &SessionRequest{}); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Expected an empty JSON object")
		return "", false
	}
	if cookie, err := r.Cookie(cookieName); err == nil {
		return cookie.Value, true
	}
	return "", true
}

// Me returns the current public account identified by the Bearer access JWT.
//
// @Summary Get the current account
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} User
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is forbidden"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/auth/me [get]
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		h.fail(w, store.ErrUnauthorized)
		return
	}
	user, err := h.service.Me(r.Context(), parts[1])
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

// UpdateProfile accepts only a name and derives the account from the Bearer access JWT.
//
// @Summary Update the account name
// @Tags profile
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param profile body ProfileRequest true "Name only; trimmed to 1-100 Unicode code points without control characters"
// @Success 200 {object} User
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Router /api/auth/profile [patch]
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		h.fail(w, store.ErrUnauthorized)
		return
	}
	var input ProfileRequest
	if err := httpx.Decode(w, r, &input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "Expected a JSON object with name only")
		return
	}
	user, err := h.service.UpdateProfile(r.Context(), parts[1], input.Name)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) setCookie(w http.ResponseWriter, raw string, expires time.Time) {
	c := &http.Cookie{Name: cookieName, Value: raw, Path: "/api/auth", HttpOnly: true,
		Secure: h.secure, SameSite: http.SameSiteLaxMode, Expires: expires.UTC(),
		MaxAge: int(expires.Sub(h.now()).Seconds())}
	if raw == "" || c.MaxAge < 1 {
		c.Value, c.MaxAge, c.Expires = "", -1, time.Unix(1, 0).UTC()
	}
	http.SetCookie(w, c)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidName):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_name", err.Error())
	case errors.Is(err, ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, store.ErrUnauthorized):
		w.Header().Set("WWW-Authenticate", "Bearer")
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Invalid credentials or token")
	case errors.Is(err, store.ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, "email_taken", "Email already registered")
	default:
		slog.Error("auth request failed", "error", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
	}
}

// AvatarFile serves validated stored images publicly, or returns 404 when storage is disabled.
//
// @Summary Download a public avatar
// @Tags profile
// @Produce image/jpeg,image/png
// @Param file path string true "Server-generated filename from avatar_url"
// @Success 200 {file} binary "JPEG or PNG image"
// @Success 206 {file} binary "Partial image for a Range request"
// @Success 304 "Image has not changed"
// @Failure 403 {object} httpx.ErrorResponse "Origin is forbidden"
// @Failure 404 {string} string "File not found (plain text)"
// @Failure 416 {string} string "Invalid byte range (plain text)"
// @Router /uploads/avatars/{file} [get]
func (h *Handler) AvatarFile(w http.ResponseWriter, r *http.Request) {
	if h.avatars == nil {
		http.NotFound(w, r)
		return
	}
	h.avatars.files.ServeHTTP(w, r)
}

// UploadAvatar accepts one JPEG/PNG of at most 5 MiB and an optional validated name.
// It saves their database changes together and removes the new file if persistence fails.
//
// @Summary Upload an avatar and optional name
// @Tags profile
// @Description Accepts exactly one avatar file and at most one name field; persists the avatar and optional name together.
// @Accept mpfd
// @Produce json
// @Security BearerAuth
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param avatar formData file true "JPEG or PNG up to 5 MiB, 4096 pixels per side and 16 megapixels"
// @Param name formData string false "Optional name, trimmed to 1-100 Unicode code points without controls"
// @Success 200 {object} User
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 413 {object} httpx.ErrorResponse "File exceeds size limit"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Failure 503 {object} httpx.ErrorResponse "Avatar storage unavailable"
// @Router /api/auth/avatar [post]
func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		h.fail(w, store.ErrUnauthorized)
		return
	}
	user, err := h.service.Me(r.Context(), parts[1])
	if err != nil {
		h.fail(w, err)
		return
	}
	if h.avatars == nil {
		httpx.WriteError(w, 503, "storage_unavailable", "Avatar storage unavailable")
		return
	}
	// Reserve 64 KiB for multipart boundaries and fields; the file itself still has a 5 MiB limit.
	r.Body = http.MaxBytesReader(w, r.Body, MaxAvatarBytes+(64<<10))
	err = r.ParseMultipartForm(MaxAvatarBytes + (64 << 10))
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.WriteError(w, 413, "avatar_too_large", "Image must not exceed 5 MiB")
		} else {
			httpx.WriteError(w, 400, "invalid_avatar", "Expected multipart form with avatar")
		}
		return
	}
	form := r.MultipartForm
	if len(form.File) != 1 || len(form.File["avatar"]) != 1 || len(form.Value) > 1 || (len(form.Value) == 1 && len(form.Value["name"]) != 1) {
		httpx.WriteError(w, 400, "invalid_avatar", "Expected avatar file and optional name only")
		return
	}
	var name *string
	if names := form.Value["name"]; len(names) == 1 {
		value, err := ValidateName(names[0])
		if err != nil {
			h.fail(w, err)
			return
		}
		name = &value
	}
	header := form.File["avatar"][0]
	if header.Size > MaxAvatarBytes {
		httpx.WriteError(w, 413, "avatar_too_large", "Image must not exceed 5 MiB")
		return
	}
	file, err := header.Open()
	if err != nil {
		h.fail(w, err)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxAvatarBytes+1))
	if err != nil {
		h.fail(w, err)
		return
	}
	if len(data) > MaxAvatarBytes {
		httpx.WriteError(w, 413, "avatar_too_large", "Image must not exceed 5 MiB")
		return
	}
	url, err := h.avatars.save(data)
	if errors.Is(err, errInvalidAvatar) {
		httpx.WriteError(w, 400, "invalid_avatar", "Use a valid JPEG or PNG, up to 4096 px and 16 megapixels")
		return
	}
	if err != nil {
		h.fail(w, err)
		return
	}
	updated, previous, err := h.service.Store.UpdateUserAvatar(r.Context(), user.ID, url, name)
	if err != nil {
		h.avatars.remove(url)
		h.fail(w, err)
		return
	}
	// Keep the previous image available until its replacement is committed.
	h.avatars.remove(previous)
	httpx.WriteJSON(w, http.StatusOK, publicUser(updated))
}

// RequireUser verifies the Bearer JWT and current account, then sets the trusted context user ID.
func (h *Handler) RequireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			h.fail(w, store.ErrUnauthorized)
			return
		}
		user, err := h.service.Me(r.Context(), parts[1])
		if err != nil {
			h.fail(w, err)
			return
		}
		next(w, r.WithContext(httpx.WithUserID(r.Context(), user.ID)))
	}
}
