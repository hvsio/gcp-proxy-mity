package config

import "errors"

var (
	ErrMissingProjectID = errors.New("GCP_PROJECT_ID is required")
	ErrMissingBucketName = errors.New("GCS_BUCKET_NAME is required")
)