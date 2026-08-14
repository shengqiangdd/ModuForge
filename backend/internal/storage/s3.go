package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
}

// NewS3Adapter creates a new S3-backed storage adapter.
func NewS3Adapter(cfg S3Config) (*S3Adapter, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
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
			ContentType: detectContentType(path),
		},
	)
	return err
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

func (s *S3Adapter) Close() error {
	return nil // minio client has no close
}

// Stat returns file metadata from S3.
func (s *S3Adapter) Stat(ctx context.Context, path string) (*FileInfo, error) {
	obj, err := s.client.StatObject(ctx, s.bucket, s.s3Path(path), minio.StatObjectOptions{})
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Path:  path,
		Size:  obj.Size,
		MTime: obj.LastModified.Format(time.RFC3339),
	}, nil
}

// ComputeSHA256 computes the SHA-256 hash of content.
func ComputeSHA256(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func detectContentType(path string) string {
	if strings.HasSuffix(path, ".html") || strings.HasSuffix(path, ".htm") {
		return "text/html; charset=utf-8"
	}
	if strings.HasSuffix(path, ".css") {
		return "text/css; charset=utf-8"
	}
	if strings.HasSuffix(path, ".js") {
		return "application/javascript"
	}
	if strings.HasSuffix(path, ".json") {
		return "application/json"
	}
	if strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".bash") {
		return "text/x-shellscript"
	}
	if strings.HasSuffix(path, ".go") {
		return "text/x-go"
	}
	if strings.HasSuffix(path, ".rs") {
		return "text/x-rust"
	}
	if strings.HasSuffix(path, ".cpp") || strings.HasSuffix(path, ".cc") || strings.HasSuffix(path, ".cxx") {
		return "text/x-c++"
	}
	if strings.HasSuffix(path, ".h") || strings.HasSuffix(path, ".hpp") {
		return "text/x-c-header"
	}
	if strings.HasSuffix(path, ".py") {
		return "text/x-python"
	}
	if strings.HasSuffix(path, ".md") {
		return "text/markdown; charset=utf-8"
	}
	if strings.HasSuffix(path, ".xml") {
		return "application/xml"
	}
	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return "text/yaml"
	}
	if strings.HasSuffix(path, ".toml") {
		return "text/toml"
	}
	if strings.HasSuffix(path, ".zip") {
		return "application/zip"
	}
	if strings.HasSuffix(path, ".png") {
		return "image/png"
	}
	if strings.HasSuffix(path, ".jpg") || strings.HasSuffix(path, ".jpeg") {
		return "image/jpeg"
	}
	if strings.HasSuffix(path, ".svg") {
		return "image/svg+xml"
	}
	return "application/octet-stream"
}