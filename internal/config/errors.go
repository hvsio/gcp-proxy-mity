package config

import "errors"

var (
	// Storage related errors
	ErrMissingProjectID  = errors.New("GCP_PROJECT_ID is required")
	ErrMissingBucketName = errors.New("GCS_BUCKET_NAME is required")

	// Database related errors
	ErrMissingInstanceConnectionName = errors.New("DB_INSTANCE_CONNECTION_NAME is required for Cloud SQL")
	ErrMissingDBHost                 = errors.New("DB_HOST is required for PostgreSQL")
	ErrMissingDatabaseName           = errors.New("DB_DATABASE_NAME is required")
	ErrMissingDBUsername             = errors.New("DB_USERNAME is required")
	ErrUnsupportedMetadataBackend    = errors.New("PHOTO_METADATA_BACKEND must be one of postgres or firestore")
	ErrMissingIAPAudience            = errors.New("IAP_AUDIENCE is required when ALLOWED_IAP_EMAILS is set")
	ErrMissingIAPAllowedEmails       = errors.New("ALLOWED_IAP_EMAILS is required when IAP_AUDIENCE is set")
)
