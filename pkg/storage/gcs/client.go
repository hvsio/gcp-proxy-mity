package gcs

import (
	"context"
	"encoding/base64"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

type Client struct {
	client     *storage.Client
	bucketName string
}

func NewClient(ctx context.Context, projectID, bucketName string, credentialsPath string) (*Client, error) {
	var opts []option.ClientOption
	if credentialsPath != "" {
		d, err := base64.StdEncoding.DecodeString(credentialsPath)
		if err != nil {
			return nil, err
		}
		opts = append(opts, option.WithCredentialsJSON(d))
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &Client{
		client:     client,
		bucketName: bucketName,
	}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) GetBucket() *storage.BucketHandle {
	return c.client.Bucket(c.bucketName)
}
