// Package storage firma gli URL di upload e download dei documenti delle
// pratiche su S3/MinIO tramite github.com/gigliofr/gokit/storagex/s3.
package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gigliofr/gokit/storagex/s3"
	"github.com/google/uuid"
)

type UploadSession struct {
	Key       string    `json:"key"`
	UploadURL string    `json:"upload_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

type DownloadSession struct {
	DownloadURL string    `json:"download_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type PresignService interface {
	PresignDocumentUpload(praticaID, fileName, contentType string, size int64) (UploadSession, error)
	PresignDocumentDownload(key, fileName string) (DownloadSession, error)
}

type disabledPresigner struct{}

func (d disabledPresigner) PresignDocumentUpload(_, _, _ string, _ int64) (UploadSession, error) {
	return UploadSession{}, errors.New("storage presign not configured")
}

func (d disabledPresigner) PresignDocumentDownload(_, _ string) (DownloadSession, error) {
	return DownloadSession{}, errors.New("storage presign not configured")
}

type MinioPresigner struct {
	p *s3.Presigner
}

func operationContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 8*time.Second)
}

// NewPresignServiceFromEnv legge S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY,
// S3_SECRET_KEY, S3_USE_SSL (e facoltativa S3_REGION) e verifica il bucket.
// Senza configurazione restituisce un servizio disattivato.
func NewPresignServiceFromEnv() (PresignService, error) {
	cfg, ok := s3.ConfigFromEnv()
	if !ok {
		return disabledPresigner{}, nil
	}
	cfg.Expires = 15 * time.Minute
	p, err := s3.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("minio client init failed: %w", err)
	}
	ctx, cancel := operationContext()
	defer cancel()
	if err := p.CheckBucket(ctx); err != nil {
		return nil, err
	}
	return &MinioPresigner{p: p}, nil
}

func (m *MinioPresigner) PresignDocumentUpload(praticaID, fileName, contentType string, size int64) (UploadSession, error) {
	if strings.TrimSpace(praticaID) == "" {
		return UploadSession{}, errors.New("pratica id is required")
	}
	safeName := s3.SafeFileName(fileName)
	if safeName == "" {
		safeName = "documento.bin"
	}
	key := s3.Key("pratiche", praticaID, "documenti", fmt.Sprintf("%s_%s", uuid.NewString(), safeName))

	ctx, cancel := operationContext()
	defer cancel()
	u, exp, err := m.p.PresignPut(ctx, key)
	if err != nil {
		return UploadSession{}, fmt.Errorf("presign failed: %w", err)
	}
	return UploadSession{Key: key, UploadURL: u, ExpiresAt: exp}, nil
}

func (m *MinioPresigner) PresignDocumentDownload(key, fileName string) (DownloadSession, error) {
	if strings.TrimSpace(key) == "" {
		return DownloadSession{}, errors.New("document key is required")
	}
	ctx, cancel := operationContext()
	defer cancel()
	u, exp, err := m.p.PresignGet(ctx, key, fileName)
	if err != nil {
		return DownloadSession{}, fmt.Errorf("presign download failed: %w", err)
	}
	return DownloadSession{DownloadURL: u, ExpiresAt: exp}, nil
}
