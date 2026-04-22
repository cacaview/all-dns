package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Storage stores files in any S3-compatible object store (AWS S3, MinIO, Ceph, etc.).
type S3Storage struct {
	client    *s3.Client
	bucket    string
	baseURL   string
	keyPrefix string
}

// S3Config holds the parameters for building an S3Storage.
type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	KeyPrefix       string
	BaseURL         string
	AccessKeyID     string
	SecretAccessKey string
}

// NewS3Storage creates an S3Storage from the provided config.
func NewS3Storage(ctx context.Context, cfg S3Config) (*S3Storage, error) {
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.Region),
	}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}
	if cfg.Endpoint != "" {
		customResolver := aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: cfg.Endpoint, SigningRegion: cfg.Region}, nil
			},
		)
		opts = append(opts, awsconfig.WithEndpointResolverWithOptions(customResolver))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.UsePathStyle = true
		}
	})

	return &S3Storage{
		client:    client,
		bucket:    cfg.Bucket,
		baseURL:   cfg.BaseURL,
		keyPrefix: cfg.KeyPrefix,
	}, nil
}

func (s *S3Storage) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	fullKey := s.keyPrefix + key
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(fullKey),
		Body:        readerFromBytes(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put object: %w", err)
	}
	if s.baseURL != "" {
		return strings.TrimRight(s.baseURL, "/") + "/" + key, nil
	}
	return "", nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.keyPrefix + key),
	})
	return err
}

type bytesReader struct{ data []byte }

func readerFromBytes(data []byte) io.Reader {
	return &bytesReader{data: data}
}

func (r *bytesReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[:0]
	return n, io.EOF
}
