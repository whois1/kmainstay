package attachments

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ErrAlreadyExists = errors.New("attachment already exists")
	ErrInvalidKey    = errors.New("invalid attachment storage key")
)

var validStorageKey = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)

type Store interface {
	Save(key string, source io.Reader) error
	Open(key string) (io.ReadCloser, error)
	Delete(key string) error
}

type Filesystem struct {
	root string
}

func NewFilesystem(root string) (*Filesystem, error) {
	if root == "" {
		return nil, errors.New("attachment path is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("create attachment directory: %w", err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return nil, fmt.Errorf("restrict attachment directory: %w", err)
	}
	return &Filesystem{root: root}, nil
}

func (filesystem *Filesystem) Save(key string, source io.Reader) error {
	if !validStorageKey.MatchString(key) {
		return ErrInvalidKey
	}
	staging, err := os.CreateTemp(filesystem.root, ".staging-")
	if err != nil {
		return fmt.Errorf("create attachment staging file: %w", err)
	}
	stagingPath := staging.Name()
	defer os.Remove(stagingPath)
	if _, err := io.Copy(staging, source); err != nil {
		_ = staging.Close()
		return fmt.Errorf("write attachment staging file: %w", err)
	}
	if err := staging.Sync(); err != nil {
		_ = staging.Close()
		return fmt.Errorf("sync attachment staging file: %w", err)
	}
	if err := staging.Close(); err != nil {
		return fmt.Errorf("close attachment staging file: %w", err)
	}
	finalPath := filepath.Join(filesystem.root, key)
	if err := os.Link(stagingPath, finalPath); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("publish attachment: %w", err)
	}
	if err := syncDirectory(filesystem.root); err != nil {
		_ = os.Remove(finalPath)
		_ = syncDirectory(filesystem.root)
		return fmt.Errorf("sync attachment directory: %w", err)
	}
	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func (filesystem *Filesystem) Open(key string) (io.ReadCloser, error) {
	if !validStorageKey.MatchString(key) {
		return nil, ErrInvalidKey
	}
	return os.Open(filepath.Join(filesystem.root, key))
}

func (filesystem *Filesystem) Delete(key string) error {
	if !validStorageKey.MatchString(key) {
		return ErrInvalidKey
	}
	if err := os.Remove(filepath.Join(filesystem.root, key)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete attachment: %w", err)
	}
	return nil
}

func (filesystem *Filesystem) Cleanup(olderThan time.Time) (int, error) {
	entries, err := os.ReadDir(filesystem.root)
	if err != nil {
		return 0, fmt.Errorf("read attachment directory: %w", err)
	}
	removed := 0
	var cleanupErrors []error
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, ".staging-") || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("inspect attachment %q: %w", name, err))
			continue
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(olderThan) {
			continue
		}
		if err := os.Remove(filepath.Join(filesystem.root, name)); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove attachment %q: %w", name, err))
			continue
		}
		removed++
	}
	return removed, errors.Join(cleanupErrors...)
}
