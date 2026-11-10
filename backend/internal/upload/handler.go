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

func NewHandler(storage *Storage) *Handler { return &Handler{storage: storage} }

// Upload accepts one arbitrary file. Route assembly applies authentication and HTTP guards.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if h.storage == nil {
		httpx.WriteError(w, 503, "storage_unavailable", "Upload storage unavailable")
		return
	}
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
