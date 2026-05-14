# ---------------------------------------------------------------------------
# Core
# ---------------------------------------------------------------------------

variable "project_id" {
  description = "GCP Project ID"
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project_id))
    error_message = "project_id must be a valid GCP project ID."
  }
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "europe-west4"

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]+$", var.region))
    error_message = "region must look like a GCP region, for example europe-west4."
  }
}

# ---------------------------------------------------------------------------
# Container image
# ---------------------------------------------------------------------------

variable "container_image" {
  description = "Immutable container image to deploy, preferably pinned to a digest."
  type        = string

  validation {
    condition     = var.container_image != "" && !endswith(var.container_image, ":latest")
    error_message = "container_image must be set and must not use the latest tag."
  }
}

# ---------------------------------------------------------------------------
# Optional database
# ---------------------------------------------------------------------------

variable "enable_database" {
  description = "Provision Cloud SQL and start the app with Cloud SQL enabled."
  type        = bool
  default     = false
}

variable "db_disk_type" {
  description = "Cloud SQL disk type. PD_HDD is the lowest-cost option for low-traffic deployments."
  type        = string
  default     = "PD_HDD"

  validation {
    condition     = contains(["PD_HDD", "PD_SSD"], var.db_disk_type)
    error_message = "db_disk_type must be either PD_HDD or PD_SSD."
  }
}

variable "db_point_in_time_recovery_enabled" {
  description = "Enable Cloud SQL point-in-time recovery. Keep false for lowest idle backup cost."
  type        = bool
  default     = false
}

variable "db_backup_retention_count" {
  description = "Number of automated Cloud SQL backups to retain. One is the lowest reasonable safety floor."
  type        = number
  default     = 1

  validation {
    condition     = var.db_backup_retention_count >= 1 && var.db_backup_retention_count <= 365
    error_message = "db_backup_retention_count must be between 1 and 365."
  }
}

variable "db_activation_policy" {
  description = "Cloud SQL activation policy. ALWAYS keeps the DB available; NEVER stops it when idle by configuration."
  type        = string
  default     = "ALWAYS"

  validation {
    condition     = contains(["ALWAYS", "NEVER"], var.db_activation_policy)
    error_message = "db_activation_policy must be either ALWAYS or NEVER."
  }
}

variable "db_max_connections" {
  description = "Maximum PostgreSQL connections opened by each Cloud Run instance."
  type        = number
  default     = 3

  validation {
    condition     = var.db_max_connections >= 1 && var.db_max_connections <= 10
    error_message = "db_max_connections must be between 1 and 10."
  }
}

# ---------------------------------------------------------------------------
# Optional private egress
# ---------------------------------------------------------------------------

variable "vpc_connector_id" {
  description = "Existing Serverless VPC Access connector ID. When set, Cloud Run uses PRIVATE_RANGES_ONLY egress."
  type        = string
  default     = ""
}

# ---------------------------------------------------------------------------
# Cloud Run access
# ---------------------------------------------------------------------------

variable "allow_public_invoker" {
  description = "Grant allUsers roles/run.invoker. Keep false when access is handled by IAM, IAP, or a load balancer."
  type        = bool
  default     = false
}

# ---------------------------------------------------------------------------
# IAP and access control
# ---------------------------------------------------------------------------

variable "allowed_iap_user_emails" {
  description = "Google account emails allowed to access the app via IAP."
  type        = list(string)
  default     = []
}

variable "iap_audience" {
  description = "IAP JWT audience for backend validation. Leave empty to skip IAP validation."
  type        = string
  default     = ""
}

# ---------------------------------------------------------------------------
# CORS
# ---------------------------------------------------------------------------

variable "cors_allowed_origins" {
  description = "Allowed CORS origins, e.g. https://album.example.com."
  type        = list(string)
  default     = []
}

# ---------------------------------------------------------------------------
# Storage retention
# ---------------------------------------------------------------------------

variable "bucket_lifecycle_delete_age_days" {
  description = "Optional number of days after which bucket objects are deleted. Null keeps objects indefinitely."
  type        = number
  default     = null

  validation {
    condition     = var.bucket_lifecycle_delete_age_days == null || var.bucket_lifecycle_delete_age_days > 0
    error_message = "bucket_lifecycle_delete_age_days must be null or greater than zero."
  }
}
