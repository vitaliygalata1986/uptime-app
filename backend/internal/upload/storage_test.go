package upload

import (
	"bytes"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestStoragePreservesFilesAndRejectsPaths(t *testing.T) {
	dir := t.TempDir()
	s, err := NewStorage(dir, "/uploads/files/")
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("<script>alert('test')</script>\x00\xff")
	url, err := s.Save(bytes.NewReader(data), ".bin")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.Save(bytes.NewReader(data), ".bin")
	if err != nil || other == url {
		t.Fatal("filenames must be unique", err)
	}
	s.Remove("/uploads/files/../" + strings.TrimPrefix(url, s.prefix))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("unsafe or corrupted download: %d %v", w.Code, w.Header())
	}
	for _, path := range []string{"/uploads/files/", "/uploads/files/../secret", "/uploads/avatars/" + strings.TrimPrefix(url, s.prefix)} {
		w = httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("invalid path: %s returned %d", path, w.Code)
		}
	}
	s.Remove(url)
	s.Remove(other)
	if _, err := s.Save(io.MultiReader(strings.NewReader("partial"), failingReader{}), ".bin"); err == nil {
		t.Fatal("reader failure ignored")
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal("partial or deleted files remain", files, err)
	}
}
