package storage

import (
	"net/url"
	"strings"
	"testing"

	"github.com/gigliofr/gokit/storagex/s3"
)

func TestMinioPresigner(t *testing.T) {
	p, err := s3.New(s3.Config{Endpoint: "minio.local:9000", Bucket: "visto", AccessKey: "a", SecretKey: "s", Region: "eu-south-1"})
	if err != nil {
		t.Fatal(err)
	}
	m := &MinioPresigner{p: p}
	up, err := m.PresignDocumentUpload("pr-1", "../Passaporto Mario.pdf", "application/pdf", 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(up.Key, "pratiche/pr-1/documenti/") || !strings.HasSuffix(up.Key, "_Passaporto Mario.pdf") || strings.Contains(up.Key, "..") {
		t.Fatalf("chiave: %s", up.Key)
	}
	if u, _ := url.Parse(up.UploadURL); u.Query().Get("X-Amz-Signature") == "" {
		t.Fatalf("URL non firmato: %s", up.UploadURL)
	}
	if _, err := m.PresignDocumentUpload(" ", "a.pdf", "", 1); err == nil {
		t.Fatal("pratica vuota")
	}
	down, err := m.PresignDocumentDownload(up.Key, "Passaporto.pdf")
	if err != nil || !strings.Contains(down.DownloadURL, "response-content-disposition") {
		t.Fatalf("download: %v %s", err, down.DownloadURL)
	}
	if _, err := (disabledPresigner{}).PresignDocumentDownload("k", ""); err == nil {
		t.Fatal("servizio disattivato")
	}
}
