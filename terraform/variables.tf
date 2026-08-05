variable "project_id" {
  description = "GCP Project ID"
  type        = string

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project_id))
    error_message = "project_id must be a valid GCP project ID."
  }
}

variable "region" {
  description = "GCP region for Cloud Run and Firestore."
  type        = string
  default     = "europe-west4"

  validation {
    condition     = can(regex("^[a-z]+-[a-z]+[0-9]+$", var.region))
    error_message = "region must look like a GCP region, for example europe-west4."
  }
}

variable "service_name" {
  description = "Cloud Run service name."
  type        = string
  default     = "gcp-proxy-mity"

  validation {
    condition     = can(regex("^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$", var.service_name))
    error_message = "service_name must be a valid Cloud Run service name."
  }
}

variable "container_image" {
  description = "Immutable container image reference pinned to a sha256 digest."
  type        = string

  validation {
    condition     = can(regex("^.+@sha256:[a-f0-9]{64}$", var.container_image))
    error_message = "container_image must be pinned to a sha256 digest."
  }
}

variable "storage_bucket_name" {
  description = "Managed private media bucket name."
  type        = string

  validation {
    condition     = var.storage_bucket_name != ""
    error_message = "storage_bucket_name must not be empty."
  }
}

variable "allowed_iap_user_emails" {
  description = "Google account emails allowed to access the backend when IAP validation is enabled."
  type        = list(string)
  default     = []
}

variable "iap_audience" {
  description = "Expected IAP JWT audience. Leave empty to skip backend IAP validation."
  type        = string
  default     = ""
}

variable "cors_allowed_origins" {
  description = "Allowed CORS origins."
  type        = list(string)
  default     = []
}

variable "frontend_service_account_email" {
  description = "Frontend Cloud Run service account allowed to invoke the backend."
  type        = string

  validation {
    condition     = can(regex("^[^@]+@[^@]+\\.iam\\.gserviceaccount\\.com$", var.frontend_service_account_email))
    error_message = "frontend_service_account_email must be a service account email."
  }
}

variable "deployment_service_account_email" {
  description = "Deployment service account allowed to run authenticated smoke checks."
  type        = string

  validation {
    condition     = can(regex("^[^@]+@[^@]+\\.iam\\.gserviceaccount\\.com$", var.deployment_service_account_email))
    error_message = "deployment_service_account_email must be a service account email."
  }
}
