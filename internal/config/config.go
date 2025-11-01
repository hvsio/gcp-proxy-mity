package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port              string
	GCPProjectID      string
	GCSBucketName     string
	GoogleCredentials string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		if _, statErr := os.Stat(".env"); statErr == nil {
			log.Printf("Warning: .env file exists but could not be loaded: %v\n", err)
		}
	}

	return &Config{
		Port:              getEnv("PORT", "8080"),
		GCPProjectID:      getEnv("GCP_PROJECT_ID", ""),
		GCSBucketName:     getEnv("GCS_BUCKET_NAME", ""),
		GoogleCredentials: getEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
	}
}

func (c *Config) Validate() error {
	if c.GCPProjectID == "" {
		return ErrMissingProjectID
	}
	if c.GCSBucketName == "" {
		return ErrMissingBucketName
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
