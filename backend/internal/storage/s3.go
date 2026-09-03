package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Adapter implements StorageAdapter using an S3-compatible object store
// (SeaweedFS, MinIO, etc.).
type S3Adapter struct {
	client *minio.Client
	bucket string
	prefix string // e.g. "projects/" — all paths are relative to this
}

// S3Config holds the connection parameters for the S3-compatible backend.
type S3Config struct {
	Endpoint  string // e.g. "seaweedfs:8333"
	AccessKey string
	SecretKey string
	Bucket    string // e.g. "moduforge"
	Prefix    string // e.g. "projects/"
	Secure    bool   // use HTTPS (false for internal)
	Region    string // default "us-east-1"
	// Retry config for transient failures
	MaxRetries int           // max retry attempts (default 3)
	RetryDelay time.Duration // base delay between retries (default 100ms)
}

// retryConfig holds retry parameters for S3 operations.
type retryConfig struct {
	maxRetries int
	retryDelay time.Duration
}

func defaultRetryConfig() retryConfig {
	return retryConfig{maxRetries: 3, retryDelay: 100 * time.Millisecond}
}

// NewS3Adapter creates a new S3-backed storage adapter.
func NewS3Adapter(cfg S3Config) (*S3Adapter, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.Endpoint == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}

	var creds *credentials.Credentials
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		// Anonymous credentials for SeaweedFS without auth
		creds = credentials.New(&anonymousProvider{})
	} else {
		creds = credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  creds,
		Secure: cfg.Secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}

	// Ensure bucket exists
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("s3 bucket check: %w", err)
	}
	if !exists {
		if err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		}); err != nil {
			return nil, fmt.Errorf("s3 create bucket: %w", err)
		}
	}

	return &S3Adapter{
		client: client,
		bucket: cfg.Bucket,
		prefix: strings.TrimSuffix(cfg.Prefix, "/"),
	}, nil
}

// s3Path prepends the prefix to the relative path.
func (s *S3Adapter) s3Path(path string) string {
	if s.prefix == "" {
		return path
	}
	return s.prefix + "/" + strings.TrimPrefix(path, "/")
}

// Write stores content at the given path. PutObject in S3 is atomic.
func (s *S3Adapter) Write(ctx context.Context, path string, content []byte) error {
	r := bytes.NewReader(content)
	_, err := s.client.PutObject(ctx, s.bucket, s.s3Path(path), r, int64(len(content)),
		minio.PutObjectOptions{
			ContentType: DetectContentType(path),
		},
	)
	return err
}

// WriteBatch stores multiple files atomically. Returns the list of paths written
// successfully and any error encountered.
func (s *S3Adapter) WriteBatch(ctx context.Context, files map[string][]byte) ([]string, error) {
	var written []string
	for path, content := range files {
		if err := s.Write(ctx, path, content); err != nil {
			slog.Warn("s3 batch write failed", "path", path, "error", err)
			continue
		}
		written = append(written, path)
	}
	return written, nil
}

// Read returns the content at the given path.
func (s *S3Adapter) Read(ctx context.Context, path string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, s.s3Path(path), minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	defer obj.Close()

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, fmt.Errorf("s3 read: %w", err)
	}
	return data, nil
}

// ReadStream returns a reader for the content at the given path.
func (s *S3Adapter) ReadStream(ctx context.Context, path string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, s.s3Path(path), minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	return obj, nil
}

// Delete removes the file at the given path.
func (s *S3Adapter) Delete(ctx context.Context, path string) error {
	return s.client.RemoveObject(ctx, s.bucket, s.s3Path(path),
		minio.RemoveObjectOptions{},
	)
}

// DeleteBatch removes multiple files. Returns the list of paths deleted
// successfully and any error encountered.
func (s *S3Adapter) DeleteBatch(ctx context.Context, paths []string) ([]string, error) {
	var deleted []string
	for _, path := range paths {
		if err := s.Delete(ctx, path); err != nil {
			slog.Warn("s3 batch delete failed", "path", path, "error", err)
			continue
		}
		deleted = append(deleted, path)
	}
	return deleted, nil
}

// Exists checks if a file exists at the given path.
func (s *S3Adapter) Exists(ctx context.Context, path string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, s.s3Path(path), minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Stat returns file metadata from S3.
func (s *S3Adapter) Stat(ctx context.Context, path string) (*FileInfo, error) {
	obj, err := s.client.StatObject(ctx, s.bucket, s.s3Path(path), minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return nil, nil // file not found
		}
		return nil, err
	}

	return &FileInfo{
		Path:  path,
		Size:  obj.Size,
		MTime: obj.LastModified.Format(time.RFC3339),
	}, nil
}

// List returns all paths under the given prefix.
func (s *S3Adapter) List(ctx context.Context, prefix string) ([]string, error) {
	searchPrefix := s.s3Path(prefix)
	var paths []string

	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    searchPrefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		// Strip the prefix to return relative paths
		relPath := strings.TrimPrefix(obj.Key, s.prefix+"/")
		paths = append(paths, relPath)
	}

	return paths, nil
}

// ListWithMetadata returns all paths and their metadata under the given prefix.
// More efficient than calling Stat on each file individually.
func (s *S3Adapter) ListWithMetadata(ctx context.Context, prefix string) ([]*FileInfo, error) {
	searchPrefix := s.s3Path(prefix)
	var files []*FileInfo

	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    searchPrefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		relPath := strings.TrimPrefix(obj.Key, s.prefix+"/")
		files = append(files, &FileInfo{
			Path:  relPath,
			Size:  obj.Size,
			MTime: obj.LastModified.Format(time.RFC3339),
		})
	}

	return files, nil
}

func (s *S3Adapter) Close() error {
	return nil // minio client has no close
}

// ComputeSHA256 computes the SHA-256 hash of content.
func ComputeSHA256(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// DetectContentType returns the MIME type for a file path.
// Uses the Go mime package with fallback to extension-based mapping.
func DetectContentType(path string) string {
	ext := filepath.Ext(path)
	if ext != "" {
		// Try Go's built-in MIME type detection
		if t := mime.TypeByExtension(ext); t != "" {
			return t
		}
	}

	// Fallback for extensions not covered by mime package
	switch strings.ToLower(ext) {
	case ".sh", ".bash":
		return "text/x-shellscript"
	case ".go":
		return "text/x-go"
	case ".rs":
		return "text/x-rust"
	case ".cpp", ".cc", ".cxx":
		return "text/x-c++"
	case ".h", ".hpp":
		return "text/x-c-header"
	case ".py":
		return "text/x-python"
	case ".toml":
		return "text/toml"
	case ".yaml", ".yml":
		return "text/yaml"
	}

	return "application/octet-stream"
}

// anonymousProvider implements credentials.Provider for anonymous S3 access.
type anonymousProvider struct{}

func (a *anonymousProvider) RetrieveWithCredContext(cc *credentials.CredContext) (credentials.Value, error) {
	return credentials.Value{}, nil
}

func (a *anonymousProvider) Retrieve() (credentials.Value, error) {
	return credentials.Value{}, nil
}

func (a *anonymousProvider) IsExpired() bool {
	return false
}
