# ---------------------------------------------------------------------------
# Core
# ---------------------------------------------------------------------------

variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "europe-west4"
}

# ---------------------------------------------------------------------------
# Container image (empty = use default from Artifact Registry)
# ---------------------------------------------------------------------------

variable "container_image" {
  description = "Container image to deploy"
  type        = string
  default     = ""
}

# ---------------------------------------------------------------------------
# IAP and access control
# ---------------------------------------------------------------------------

variable "allowed_iap_user_email" {
  description = "Google account email allowed to access the app via IAP."
  type        = string
  default     = ""
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
  description = "Comma-separated list of allowed CORS origins (e.g. https://album.example.com)."
  type        = string
  default     = ""
}
