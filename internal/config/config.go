package config

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Storage  StorageConfig
	IAP      IAPConfig
	CORS     CORSConfig
}

// CORSConfig holds Cross-Origin Resource Sharing settings.
type CORSConfig struct {
	AllowedOrigins []string
}

// IAPConfig holds Identity-Aware Proxy JWT validation settings.
// When IAPAudience is set, the backend requires X-Goog-IAP-JWT-Assertion and validates it.
type IAPConfig struct {
	Audience       string   // Expected JWT audience, e.g. /projects/PROJECT_NUMBER/global/backendServices/BACKEND_SERVICE_ID
	AllowedEmails  []string // Allowed Google account emails (at least one must match JWT email claim)
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port string
}

// DatabaseConfig holds database-related configuration
type DatabaseConfig struct {
	Type                   string // "cloudsql" or "postgres"
	InstanceConnectionName string // For Cloud SQL: projects/PROJECT_ID/regions/REGION/instances/INSTANCE_ID
	DatabaseName           string
	Username               string
	Password               string
	Host                   string // For standard PostgreSQL
	Port                   int    // For standard PostgreSQL
	SSLMode                string // For standard PostgreSQL
	MaxConnections         int32
	MaxIdleTime           time.Duration
	MaxLifetime           time.Duration
	DSN                   string // Direct DSN override
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	GCPProjectID      string
	GCSBucketName     string
	GoogleCredentials string
}

// Load loads configuration from environment variables and .env file
func Load() *Config {
	if err := godotenv.Load(); err != nil {
		if _, statErr := os.Stat(".env"); statErr == nil {
			log.Printf("Warning: .env file exists but could not be loaded: %v\n", err)
		}
	}

	return &Config{
		Server: ServerConfig{
			Port: getEnv("PORT", "8080"),
		},
		Database: DatabaseConfig{
			Type:                   getEnv("DB_TYPE", "postgres"),
			InstanceConnectionName: getEnv("DB_INSTANCE_CONNECTION_NAME", ""),
			DatabaseName:           getEnv("DB_DATABASE_NAME", "gcp_proxy"),
			Username:               getEnv("DB_USERNAME", "postgres"),
			Password:               getEnv("DB_PASSWORD", ""),
			Host:                   getEnv("DB_HOST", "localhost"),
			Port:                   getEnvAsInt("DB_PORT", 5432),
			SSLMode:                getEnv("DB_SSL_MODE", "disable"),
			MaxConnections:         getEnvAsInt32("DB_MAX_CONNECTIONS", 10),
			MaxIdleTime:           getEnvAsDuration("DB_MAX_IDLE_TIME", "30m"),
			MaxLifetime:           getEnvAsDuration("DB_MAX_LIFETIME", "1h"),
			DSN:                   getEnv("DATABASE_URL", ""), // Override with direct DSN
		},
		Storage: StorageConfig{
			GCPProjectID:      getEnv("GCP_PROJECT_ID", ""),
			GCSBucketName:     getEnv("GCS_BUCKET_NAME", ""),
			GoogleCredentials: getEnv("GOOGLE_APPLICATION_CREDENTIALS", ""),
		},
		IAP: IAPConfig{
			Audience:      getEnv("IAP_AUDIENCE", ""),
			AllowedEmails: getEnvAsSlice("ALLOWED_IAP_EMAILS", ","),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvAsSlice("CORS_ALLOWED_ORIGINS", ","),
		},
	}
}

// Validate checks if required configuration values are present
func (c *Config) Validate() error {
	if c.Storage.GCPProjectID == "" {
		return ErrMissingProjectID
	}
	if c.Storage.GCSBucketName == "" {
		return ErrMissingBucketName
	}
	
	// Database validation
	if c.Database.Type == "cloudsql" {
		if c.Database.InstanceConnectionName == "" {
			return ErrMissingInstanceConnectionName
		}
	} else if c.Database.Type == "postgres" && c.Database.DSN == "" {
		if c.Database.Host == "" {
			return ErrMissingDBHost
		}
	}
	
	if c.Database.DatabaseName == "" {
		return ErrMissingDatabaseName
	}
	if c.Database.Username == "" {
		return ErrMissingDBUsername
	}
	
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvAsInt32(key string, defaultValue int32) int32 {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseInt(value, 10, 32); err == nil {
			return int32(parsed)
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue string) time.Duration {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	// Parse default value
	if parsed, err := time.ParseDuration(defaultValue); err == nil {
		return parsed
	}
	return 0
}

func getEnvAsSlice(key, sep string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}
	var out []string
	for _, s := range splitAndTrim(value, sep) {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func splitAndTrim(s, sep string) []string {
	var out []string
	for _, part := range strings.Split(s, sep) {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}
