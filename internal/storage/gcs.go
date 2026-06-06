package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"

	cloudstorage "cloud.google.com/go/storage"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GCSStore struct {
	bucketName              string
	signedURLServiceAccount string
	client                  *cloudstorage.Client
	bucket                  *cloudstorage.BucketHandle
	iamCredentialsService   *iamcredentials.Service
}

func NewGCSStore(ctx context.Context, bucketName string, credentials string, signedURLServiceAccount string) (*GCSStore, error) {
	var opts []option.ClientOption
	if credentials != "" {
		d, err := base64.StdEncoding.DecodeString(credentials)
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithCredentialsJSON(d))
	}

	client, err := cloudstorage.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	iamService, err := iamcredentials.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &GCSStore{
		bucketName:              bucketName,
		signedURLServiceAccount: signedURLServiceAccount,
		client:                  client,
		bucket:                  client.Bucket(bucketName),
		iamCredentialsService:   iamService,
	}, nil
}

func (s *GCSStore) Close() error {
	return s.client.Close()
}

func (s *GCSStore) List(ctx context.Context, prefix string) ([]ObjectMetadata, error) {
	it := s.bucket.Objects(ctx, &cloudstorage.Query{Prefix: prefix})
	files := make([]ObjectMetadata, 0)

	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, mapGCSError("list objects", err)
		}
		files = append(files, ObjectMetadata{
			Path:        attrs.Name,
			ContentType: attrs.ContentType,
			Size:        attrs.Size,
			UpdatedAt:   attrs.Updated,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})
	return files, nil
}

func (s *GCSStore) Open(ctx context.Context, path string) (*FileStream, error) {
	obj := s.bucket.Object(path)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, mapGCSError("get object attributes", err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, mapGCSError("open object reader", err)
	}

	return &FileStream{
		Path:        path,
		ContentType: attrs.ContentType,
		Size:        attrs.Size,
		Body:        reader,
	}, nil
}

func (s *GCSStore) Write(ctx context.Context, path string, contentType string, body io.Reader) (*ObjectMetadata, error) {
	obj := s.bucket.Object(path)
	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := io.Copy(writer, body); err != nil {
		_ = writer.Close()
		return nil, mapGCSError("write object", err)
	}
	if err := writer.Close(); err != nil {
		return nil, mapGCSError("close object writer", err)
	}

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, mapGCSError("get written object attributes", err)
	}

	return &ObjectMetadata{
		Path:        attrs.Name,
		ContentType: attrs.ContentType,
		Size:        attrs.Size,
		UpdatedAt:   attrs.Updated,
	}, nil
}

func (s *GCSStore) SignedURL(ctx context.Context, path string, method string, expires time.Duration) (string, error) {
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	if s.signedURLServiceAccount == "" {
		return "", errors.New("signed URL service account email is not configured")
	}

	url, err := cloudstorage.SignedURL(s.bucketName, path, &cloudstorage.SignedURLOptions{
		GoogleAccessID: s.signedURLServiceAccount,
		Method:         method,
		Expires:        time.Now().Add(expires),
		Scheme:         cloudstorage.SigningSchemeV4,
		SignBytes: func(payload []byte) ([]byte, error) {
			resp, err := s.iamCredentialsService.Projects.ServiceAccounts.SignBlob(
				"projects/-/serviceAccounts/"+s.signedURLServiceAccount,
				&iamcredentials.SignBlobRequest{Payload: base64.StdEncoding.EncodeToString(payload)},
			).Context(ctx).Do()
			if err != nil {
				return nil, err
			}
			return base64.StdEncoding.DecodeString(resp.SignedBlob)
		},
	})
	if err != nil {
		return "", fmt.Errorf("sign object URL: %w", err)
	}

	return url, nil
}

func mapGCSError(action string, err error) error {
	if errors.Is(err, cloudstorage.ErrObjectNotExist) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", action, err)
}
