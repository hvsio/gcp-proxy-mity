package storage

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"

	cloudstorage "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GCSStore struct {
	client *cloudstorage.Client
	bucket *cloudstorage.BucketHandle
}

func NewGCSStore(ctx context.Context, bucketName string, credentials string) (*GCSStore, error) {
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

	return &GCSStore{
		client: client,
		bucket: client.Bucket(bucketName),
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

func mapGCSError(action string, err error) error {
	if errors.Is(err, cloudstorage.ErrObjectNotExist) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", action, err)
}
