package upload

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var filename = regexp.MustCompile(`^[A-Za-z0-9_-]{43}\.(bin|jpg|png)$`)

// Storage owns local files. Callers validate content before choosing an image extension.
type Storage struct {
	dir    string
	prefix string
}

func NewStorage(dir, prefix string) (*Storage, error) {
	if !strings.HasPrefix(prefix, "/uploads/") || !strings.HasSuffix(prefix, "/") {
		return nil, errors.New("invalid upload URL prefix")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Storage{dir: dir, prefix: prefix}, nil
}

// Save uses a random server filename and removes partially written files on failure.
// A .bin extension stores any file type without enabling inline browser execution.
func (s *Storage) Save(source io.Reader, extension string) (string, error) {
	if extension != ".bin" && extension != ".jpg" && extension != ".png" {
		return "", errors.New("invalid storage extension")
	}
	var random [32]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	name := base64.RawURLEncoding.EncodeToString(random[:]) + extension
	path := filepath.Join(s.dir, name)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(file, source)
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return s.prefix + name, nil
}

func (s *Storage) name(url string) (string, bool) {
	name, ok := strings.CutPrefix(url, s.prefix)
	return name, ok && filename.MatchString(name)
}

func (s *Storage) Remove(url string) {
	name, ok := s.name(url)
	if !ok {
		return
	}
	if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Error("remove upload failed", "error", err)
	}
}

func (s *Storage) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name, ok := s.name(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(filepath.Join(s.dir, name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	switch filepath.Ext(name) {
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg":
		w.Header().Set("Content-Type", "image/jpeg")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	}
	http.ServeContent(w, r, name, info.ModTime(), file)
}
