package config

import "testing"

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
