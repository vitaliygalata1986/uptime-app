package auth

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestAvatarStorageSanitizesAndRemoves(t *testing.T) {
	dir := t.TempDir()
	storage, err := NewAvatarStorage(dir)
	if err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	data.WriteString("untrusted trailing content")
	url, err := storage.save(data.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("files: %v %v", files, err)
	}
	saved, err := os.ReadFile(filepath.Join(dir, files[0].Name()))
	if err != nil || bytes.Contains(saved, []byte("untrusted")) {
		t.Fatal("image not re-encoded", err)
	}
	storage.remove("/uploads/avatars/../" + files[0].Name())
	if _, err := os.Stat(filepath.Join(dir, files[0].Name())); err != nil {
		t.Fatal("invalid path removed file")
	}
	storage.remove(url)
	files, err = os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatalf("old image remains: %v %v", files, err)
	}
}
