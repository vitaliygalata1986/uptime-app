package auth

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const MaxAvatarBytes = 5 << 20
const avatarPrefix = "/uploads/avatars/"

var avatarFilename = regexp.MustCompile(`^[A-Za-z0-9_-]{43}\.(jpg|png)$`)
var errInvalidAvatar = errors.New("invalid avatar image")

// AvatarStorage stores only decoded and re-encoded JPEG/PNG files, never client filenames.
type AvatarStorage struct{ dir string }

func NewAvatarStorage(dir string) (*AvatarStorage, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &AvatarStorage{dir: dir}, nil
}

func (s *AvatarStorage) save(data []byte) (string, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 4096 || cfg.Height > 4096 || cfg.Width*cfg.Height > 16_000_000 {
		return "", errInvalidAvatar
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", errInvalidAvatar
	}
	token, err := RandomToken()
	if err != nil {
		return "", err
	}
	extension := ".png"
	if format == "jpeg" {
		extension = ".jpg"
	}
	name := token + extension
	file, err := os.OpenFile(filepath.Join(s.dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	if format == "jpeg" {
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(file, img)
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(filepath.Join(s.dir, name))
		return "", err
	}
	return avatarPrefix + name, nil
}

func (s *AvatarStorage) remove(url string) {
	if !strings.HasPrefix(url, avatarPrefix) {
		return
	}
	name := strings.TrimPrefix(url, avatarPrefix)
	if !avatarFilename.MatchString(name) {
		return
	}
	if err := os.Remove(filepath.Join(s.dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Error("remove avatar failed", "error", err)
	}
}
