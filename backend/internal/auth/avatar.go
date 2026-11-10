package auth

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"image/png"

	"uptime-app/backend/internal/upload"
)

const MaxAvatarBytes = 5 << 20
const avatarPrefix = "/uploads/avatars/"

var errInvalidAvatar = errors.New("invalid avatar image")

// AvatarStorage validates images before writing them through shared file storage.
type AvatarStorage struct{ files *upload.Storage }

func NewAvatarStorage(dir string) (*AvatarStorage, error) {
	files, err := upload.NewStorage(dir, avatarPrefix)
	if err != nil {
		return nil, err
	}
	return &AvatarStorage{files: files}, nil
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
	extension := ".png"
	var encoded bytes.Buffer
	if format == "jpeg" {
		extension = ".jpg"
		err = jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 90})
	} else {
		err = png.Encode(&encoded, img)
	}
	if err != nil {
		return "", err
	}
	return s.files.Save(&encoded, extension)
}

func (s *AvatarStorage) remove(url string) {
	s.files.Remove(url)
}
