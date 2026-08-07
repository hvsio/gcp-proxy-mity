package config

import "testing"

func TestValidateAllowsLocalDevelopmentWithoutIAP(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			GCPProjectID:  "project-id",
			GCSBucketName: "bucket",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRequiresIAPAudienceWhenAllowedEmailsConfigured(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			GCPProjectID:  "project-id",
			GCSBucketName: "bucket",
		},
		IAP: IAPConfig{
			AllowedEmails: []string{"owner@example.com"},
		},
	}

	if err := cfg.Validate(); err != ErrMissingIAPAudience {
		t.Fatalf("Validate() error = %v, want %v", err, ErrMissingIAPAudience)
	}
}

func TestValidateRequiresIAPAllowedEmailsWhenAudienceConfigured(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			GCPProjectID:  "project-id",
			GCSBucketName: "bucket",
		},
		IAP: IAPConfig{
			Audience: "/projects/123/global/backendServices/456",
		},
	}

	if err := cfg.Validate(); err != ErrMissingIAPAllowedEmails {
		t.Fatalf("Validate() error = %v, want %v", err, ErrMissingIAPAllowedEmails)
	}
}

func TestValidateAllowsCompleteIAPConfig(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			GCPProjectID:  "project-id",
			GCSBucketName: "bucket",
		},
		IAP: IAPConfig{
			Audience:      "/projects/123/global/backendServices/456",
			AllowedEmails: []string{"owner@example.com"},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateSkipsDatabaseRequirementsWhenDisabled(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			GCPProjectID:  "project-id",
			GCSBucketName: "bucket",
		},
		Database: DatabaseConfig{
			Enabled: false,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRequiresCloudSQLConnectionNameWhenDatabaseEnabled(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			GCPProjectID:  "project-id",
			GCSBucketName: "bucket",
		},
		Database: DatabaseConfig{
			Enabled:      true,
			Type:         "cloudsql",
			DatabaseName: "gcp_proxy",
			Username:     "gcp_proxy_app",
		},
	}

	if err := cfg.Validate(); err != ErrMissingInstanceConnectionName {
		t.Fatalf("Validate() error = %v, want %v", err, ErrMissingInstanceConnectionName)
	}
}

func TestValidateAllowsFirestoreMetadataWithoutPostgresSettings(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			GCPProjectID:  "project-id",
			GCSBucketName: "bucket",
		},
		Database: DatabaseConfig{
			Enabled: true,
		},
		Metadata: MetadataConfig{
			Backend:           "firestore",
			FirestoreDatabase: "(default)",
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsUnsupportedMetadataBackend(t *testing.T) {
	cfg := &Config{
		Storage: StorageConfig{
			GCPProjectID:  "project-id",
			GCSBucketName: "bucket",
		},
		Database: DatabaseConfig{
			Enabled:      true,
			DatabaseName: "gcp_proxy",
			Username:     "gcp_proxy_app",
		},
		Metadata: MetadataConfig{
			Backend: "unknown",
		},
	}

	if err := cfg.Validate(); err != ErrUnsupportedMetadataBackend {
		t.Fatalf("Validate() error = %v, want %v", err, ErrUnsupportedMetadataBackend)
	}
}
