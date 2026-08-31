package storage

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// Regressão do bug real encontrado testando download pelo browser
// (não só "sobe sem erro"): com o SDK aws-sdk-go-v2 recente, o default
// de ResponseChecksumValidation assina x-amz-checksum-mode como
// SignedHeader em PresignGetObject. MinIO rejeita qualquer requisição
// real feita contra essa URL (browser ou curl) com "AccessDenied: There
// were headers present in the request which were not signed", porque
// nada envia esse header de volta. Skip automático sem MinIO no ar —
// mesmo padrão do runner_integration_test.go do fiapx-processing-worker.
func TestPresignGet_AgainstRealMinIO_URLIsUsableWithoutExtraHeaders(t *testing.T) {
	ctx := context.Background()
	const (
		endpoint  = "http://localhost:9000"
		bucket    = "fiapx-videos-dev"
		accessKey = "minioadmin"
		secretKey = "minioadmin"
	)

	s3, err := NewS3(ctx, bucket, "us-east-1", endpoint, endpoint, accessKey, secretKey)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}
	if err := s3.Healthy(ctx); err != nil {
		t.Skipf("MinIO not reachable at %s — skipping integration test: %v", endpoint, err)
	}

	key := "test/" + uuid.NewString() + ".txt"
	body := strings.NewReader("presign regression test fixture")
	if err := s3.Upload(ctx, key, body, int64(body.Len())); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	url, err := s3.PresignGet(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if strings.Contains(url, "checksum-mode") {
		t.Fatalf("presigned URL signs x-amz-checksum-mode, which MinIO rejects on the real request: %s", url)
	}

	// A requisição real (browser, curl) nunca envia headers extras além
	// dos padrão — reproduz isso literalmente, sem Authorization nem
	// nada customizado, exatamente como um clique em "baixar" faria.
	resp, err := http.Get(url) //nolint:noctx // é exatamente isso que queremos simular: um GET simples, sem headers extras
	if err != nil {
		t.Fatalf("GET presigned URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET presigned URL = %d, want 200 (MinIO body: see Content-Length %d)", resp.StatusCode, resp.ContentLength)
	}
}
