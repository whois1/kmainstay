package attachments_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kmainstay/internal/attachments"
)

func TestFilesystem_SaveOpenAndDeleteImmutableObject(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")
	store, err := attachments.NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	key := "0123456789abcdef"
	content := []byte("image bytes")
	if err := store.Save(key, bytes.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(key, bytes.NewReader([]byte("replacement"))); !errors.Is(err, attachments.ErrAlreadyExists) {
		t.Fatalf("second save error = %v, want ErrAlreadyExists", err)
	}
	reader, err := store.Open(key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("content = %q, err=%v", got, err)
	}
	info, err := os.Stat(filepath.Join(root, key))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
	if err := store.Delete(key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(key); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("open deleted object error = %v", err)
	}
}

func TestNewFilesystem_RestrictsExistingDirectoryPermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "uploads")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := attachments.NewFilesystem(root); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o, want 700", info.Mode().Perm())
	}
}

func TestFilesystem_RejectsStorageKeysThatEscapeRoot(t *testing.T) {
	store, err := attachments.NewFilesystem(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "../outside", "nested/object", ".staging-object"} {
		if err := store.Save(key, bytes.NewReader(nil)); !errors.Is(err, attachments.ErrInvalidKey) {
			t.Errorf("Save(%q) error = %v, want ErrInvalidKey", key, err)
		}
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestFilesystem_SaveFailureLeavesNoFinalOrStagingFile(t *testing.T) {
	root := t.TempDir()
	store, err := attachments.NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save("0123456789abcdef", failingReader{}); err == nil {
		t.Fatal("Save succeeded")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("files after failure = %v", entries)
	}
}

func TestFilesystem_CleanupRemovesOnlyOldStagingFiles(t *testing.T) {
	root := t.TempDir()
	filesystem, err := attachments.NewFilesystem(root)
	if err != nil {
		t.Fatal(err)
	}
	validKey := "valid_object_1234"
	orphanKey := "orphan_object_123"
	recentOrphanKey := "recent_object_123"
	for _, key := range []string{validKey, orphanKey, recentOrphanKey} {
		if err := filesystem.Save(key, bytes.NewBufferString(key)); err != nil {
			t.Fatal(err)
		}
	}
	stagingPath := filepath.Join(root, ".staging-crashed")
	if err := os.WriteFile(stagingPath, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range []string{orphanKey, ".staging-crashed"} {
		if err := os.Chtimes(filepath.Join(root, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := filesystem.Cleanup(time.Now().Add(-24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	for _, name := range []string{validKey, orphanKey, recentOrphanKey} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("preserved %s: %v", name, err)
		}
	}
}
