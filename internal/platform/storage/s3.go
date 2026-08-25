// Package storage wraps the AWS S3 client used to store raw uploaded
// videos and the resulting frame-zip files.
package storage

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// NewS3 talks to real AWS S3 when endpoint/accessKey/secretKey are all
// empty (the normal production path — credentials come from the EKS
// node/IRSA role via the SDK's default chain). When endpoint is set, it
// points at an S3-compatible service instead (MinIO for local
// docker-compose) using static credentials and path-style addressing,
// which MinIO requires and real AWS S3 does not.
//
// presignEndpoint overrides the host baked into presigned URLs only. It
// must differ from endpoint in docker-compose: the upload/download calls
// this process makes run inside the Docker network (endpoint =
// "http://minio:9000"), but a presigned URL is handed to an external
// caller (Postman, a browser) that resolves "minio" nowhere — it needs
// "http://localhost:9000" instead. In production both are empty and the
// distinction doesn't matter.
func NewS3(ctx context.Context, bucket, region, endpoint, presignEndpoint, accessKey, secretKey string) (*S3, error) {
	if bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is empty")
	}

	opts := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if endpoint != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})

	presignClient := s3.NewPresignClient(client)
	if presignEndpoint != "" && presignEndpoint != endpoint {
		presignTarget := s3.NewFromConfig(cfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(presignEndpoint)
			o.UsePathStyle = true
		})
		presignClient = s3.NewPresignClient(presignTarget)
	}

	return &S3{client: client, presign: presignClient, bucket: bucket}, nil
}

// Upload streams body to S3 under key. contentLength must be accurate —
// the SDK does not buffer the whole body to compute it.
func (s *S3) Upload(ctx context.Context, key string, body io.Reader, contentLength int64) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(contentLength),
	})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

// PresignGet returns a time-limited URL the caller can download directly
// from S3, without proxying the bytes through this service.
func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	out, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign %s: %w", key, err)
	}
	return out.URL, nil
}

func (s *S3) Healthy(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("s3 client not initialized")
	}
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	return err
}
