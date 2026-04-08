package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type UploadSession struct {
	Key       string    `json:"key"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PresignService interface {
	PresignDocumentUpload(praticaID, fileName, contentType string, size int64) (UploadSession, error)
}

type disabledPresigner struct{}

func (d disabledPresigner) PresignDocumentUpload(_, _, _ string, _ int64) (UploadSession, error) {
	return UploadSession{}, errors.New("storage presign not configured")
}

type MinioPresigner struct {
	client  *minio.Client
	bucket  string
	expires time.Duration
}

func NewPresignServiceFromEnv() (PresignService, error) {
	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
	accessKey := strings.TrimSpace(os.Getenv("S3_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("S3_SECRET_KEY"))

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return disabledPresigner{}, nil
	}

	useSSL := true
	if raw := strings.TrimSpace(os.Getenv("S3_USE_SSL")); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err == nil {
			useSSL = v
		}
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client init failed: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		return nil, fmt.Errorf("bucket check failed: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("bucket not found: %s", bucket)
	}

	return &MinioPresigner{client: client, bucket: bucket, expires: 15 * time.Minute}, nil
}

func (m *MinioPresigner) PresignDocumentUpload(praticaID, fileName, contentType string, size int64) (UploadSession, error) {
	if strings.TrimSpace(praticaID) == "" {
		return UploadSession{}, errors.New("pratica id is required")
	}
	safeName := sanitizeFileName(fileName)
	if safeName == "" {
		safeName = "documento.bin"
	}
	key := path.Join("pratiche", praticaID, "documenti", fmt.Sprintf("%s_%s", uuid.NewString(), safeName))

	url, err := m.client.PresignedPutObject(context.Background(), m.bucket, key, m.expires)
	if err != nil {
		return UploadSession{}, fmt.Errorf("presign failed: %w", err)
	}
	return UploadSession{Key: key, UploadURL: url.String(), ExpiresAt: time.Now().UTC().Add(m.expires)}, nil
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "..", "_")
	return name
}
