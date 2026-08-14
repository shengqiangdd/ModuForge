package storage

import (
	"context"
	"io"
)

// StorageAdapter is the single source of truth for file storage.
// All file operations go through this interface, eliminating the
// dual-write issue between DB and disk.
type StorageAdapter interface {
	// Write stores content at the given path.
	// The implementation must ensure atomic writes (no partial writes).
	Write(ctx context.Context, path string, content []byte) error

	// Read returns the content at the given path.
	Read(ctx context.Context, path string) ([]byte, error)

	// ReadStream returns a reader for the content at the given path.
	// The caller must close the reader.
	ReadStream(ctx context.Context, path string) (io.ReadCloser, error)

	// Delete removes the file at the given path.
	Delete(ctx context.Context, path string) error

	// Exists checks if a file exists at the given path.
	Exists(ctx context.Context, path string) (bool, error)

	// List returns all paths under the given prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// Close cleans up any resources held by the adapter.
	Close() error
}

// FileInfo holds metadata about a stored file.
type FileInfo struct {
	Path   string
	Size   int64
	SHA256 string
	MTime  string
}