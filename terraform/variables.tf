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
