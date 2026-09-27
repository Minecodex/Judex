// SPDX-License-Identifier: Apache-2.0

// Package objectstore wraps the S3-compatible object storage (SeaweedFS or
// external) with the constraints of docs/plans/v1/04 §3: server-assigned
// keys, per-project prefixes, streamed bodies (no full-file buffering).
package objectstore

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	apierrors "github.com/kakj-go/Judex/internal/platform/errors"
)

// Options mirrors the Object config contract (docs/plans/v1/11 §2).
type Options struct {
	Endpoint        string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PathStyle       bool
}

type Store struct {
	client *s3.Client
	bucket string
}

func New(ctx context.Context, opts Options) (*Store, error) {
	if opts.Endpoint == "" || opts.Bucket == "" {
		return nil, apierrors.New(apierrors.DependencyDown, "object storage endpoint/bucket required")
	}
	creds := credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, "")
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(firstNonEmpty(opts.Region, "us-east-1")),
		awsconfig.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, apierrors.New(apierrors.DependencyDown, "object storage config failed").Wrap(err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(opts.Endpoint)
		o.UsePathStyle = opts.PathStyle
	})
	return &Store{client: client, bucket: opts.Bucket}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// ObjectKey builds the server-assigned key: projects/<id>/<kind>/<uuid>/<name>.
// User filenames never shape server paths (04 §3).
func ObjectKey(projectID, kind, id string, name string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r >= 'A' && r <= 'Z':
			return r
		case r == '.', r == '-', r == '_':
			return r
		default:
			return '_'
		}
	}, name)
	if len(safe) > 80 {
		safe = safe[len(safe)-80:]
	}
	return fmt.Sprintf("projects/%s/%s/%s/%s", projectID, kind, id, safe)
}

// Put streams a reader into the object key.
func (s *Store) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return apierrors.New(apierrors.DependencyDown, "object upload failed").WithRetryable(true).Wrap(err)
	}
	return nil
}

// Get returns the object body for download proxying (Range handled by the
// HTTP layer via passthrough headers when needed).
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, apierrors.New(apierrors.DependencyDown, "object download failed").WithRetryable(true).Wrap(err)
	}
	return out.Body, nil
}

// Delete removes an object (only for orphan cleanup of never-registered data).
func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return apierrors.New(apierrors.DependencyDown, "object delete failed").Wrap(err)
	}
	return nil
}

// Head checks existence and size.
func (s *Store) Head(ctx context.Context, key string) (int64, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key),
	})
	if err != nil {
		return 0, apierrors.New(apierrors.DependencyDown, "object stat failed").Wrap(err)
	}
	if out.ContentLength != nil {
		return *out.ContentLength, nil
	}
	return 0, nil
}
