package upload

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"uptime-app/backend/internal/httpx"
)

const MaxFileBytes = 20 << 20

type File struct {
	URL         string `json:"url"`
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

type Handler struct{ storage *Storage }

// NewHandler binds upload and public download endpoints to optional file storage.
func NewHandler(storage *Storage) *Handler { return &Handler{storage: storage} }

// Upload accepts exactly one file of any type up to 20 MiB, with no extra fields.
// Route assembly must apply authentication and HTTP guards before calling it.
//
// @Summary Upload a file
// @Tags uploads
// @Accept mpfd
// @Produce json
// @Security BearerAuth
// @Param Origin header string true "An origin configured in ALLOWED_ORIGINS" example(http://localhost:3000)
// @Param file formData file true "One file of any type, up to 20 MiB; no extra fields"
// @Success 201 {object} File
// @Failure 400 {object} httpx.ErrorResponse "Invalid request or parameters"
// @Failure 401 {object} httpx.ErrorResponse "Invalid credentials or token"
// @Failure 403 {object} httpx.ErrorResponse "Origin is missing or forbidden"
// @Failure 413 {object} httpx.ErrorResponse "File exceeds size limit"
// @Failure 429 {object} httpx.ErrorResponse "Rate limit exceeded; see Retry-After"
// @Failure 500 {object} httpx.ErrorResponse "Internal server error"
// @Failure 503 {object} httpx.ErrorResponse "File storage unavailable"
// @Router /api/uploads [post]
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, 503, "storage_unavailable", "Upload storage unavailable")
		return
	}
	// Multipart boundaries need their own allowance beyond the 20 MiB file limit.
	r.Body = http.MaxBytesReader(w, r.Body, MaxFileBytes+(64<<10))
	err := r.ParseMultipartForm(1 << 20)
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			httpx.WriteError(w, 413, "file_too_large", "File must not exceed 20 MiB")
		} else {
			httpx.WriteError(w, 400, "invalid_upload", "Expected multipart form with one file")
		}
		return
	}
	form := r.MultipartForm
	if form == nil || len(form.File) != 1 || len(form.File["file"]) != 1 || len(form.Value) != 0 {
		httpx.WriteError(w, 400, "invalid_upload", "Expected one file field only")
		return
	}
	header := form.File["file"][0]
	if header.Size > MaxFileBytes {
		httpx.WriteError(w, 413, "file_too_large", "File must not exceed 20 MiB")
		return
	}
	file, err := header.Open()
	if err != nil {
		h.fail(w, err)
		return
	}
	defer file.Close()
	var sample [512]byte
	n, err := io.ReadFull(file, sample[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		h.fail(w, err)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		h.fail(w, err)
		return
	}
	url, err := h.storage.Save(file, ".bin")
	if err != nil {
		h.fail(w, err)
		return
	}
	// The original name is response metadata only; never use it as a disk path.
	name := header.Filename
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	httpx.WriteJSON(w, http.StatusCreated, File{URL: url, Name: name, Size: header.Size, ContentType: http.DetectContentType(sample[:n])})
}

// File serves public attachments, or returns 404 when storage is disabled.
//
// @Summary Download a public attachment
// @Tags uploads
// @Produce application/octet-stream
// @Param file path string true "Server-generated filename from the upload URL"
// @Success 200 {file} binary "Attachment"
// @Success 206 {file} binary "Partial attachment for a Range request"
// @Success 304 "Attachment has not changed"
// @Failure 403 {object} httpx.ErrorResponse "Origin is forbidden"
// @Failure 404 {string} string "File not found (plain text)"
// @Failure 416 {string} string "Invalid byte range (plain text)"
// @Router /uploads/files/{file} [get]
func (h *Handler) File(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		http.NotFound(w, r)
		return
	}
	h.storage.ServeHTTP(w, r)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	slog.Error("upload request failed", "error", err)
	httpx.WriteError(w, 500, "internal_error", "Internal server error")
}
