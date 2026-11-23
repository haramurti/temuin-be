// Package storage stores uploaded images on local disk (swap for S3/GCS on deploy).
package storage

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

var allowed = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type LocalStorage struct {
	dir     string
	baseURL string
	maxSize int64
}

func NewLocal(dir, baseURL string, maxMB int) (*LocalStorage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &LocalStorage{dir: dir, baseURL: strings.TrimRight(baseURL, "/"), maxSize: int64(maxMB) << 20}, nil
}

var (
	ErrTooLarge    = fmt.Errorf("file too large")
	ErrInvalidType = fmt.Errorf("only jpeg, png, webp images are allowed")
)

// SaveImage validates content by sniffing bytes (not trusting the header) and
// stores it under <dir>/<folder>/<uuid>.<ext>. Returns the public URL.
func (s *LocalStorage) SaveImage(folder string, r io.Reader, size int64) (string, error) {
	if size > s.maxSize {
		return "", ErrTooLarge
	}
	data, err := io.ReadAll(io.LimitReader(r, s.maxSize+1))
	if err != nil {
		return "", err
	}
	if int64(len(data)) > s.maxSize {
		return "", ErrTooLarge
	}
	ext, ok := allowed[http.DetectContentType(data)]
	if !ok {
		return "", ErrInvalidType
	}
	sub := filepath.Join(s.dir, folder)
	if err := os.MkdirAll(sub, 0o755); err != nil {
		return "", err
	}
	name := uuid.NewString() + ext
	if err := os.WriteFile(filepath.Join(sub, name), data, 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/uploads/%s/%s", s.baseURL, folder, name), nil
}
