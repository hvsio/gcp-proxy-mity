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
# Container images (empty = use default from Artifact Registry)
# ---------------------------------------------------------------------------

variable "backend_container_image" {
  description = "Backend (gcp-proxy-mity) container image. Empty = use default from Artifact Registry."
  type        = string
  default     = ""
}

variable "frontend_container_image" {
  description = "Frontend (uni-album) container image. Empty = use default from Artifact Registry."
  type        = string
  default     = ""
}

# ---------------------------------------------------------------------------
# IAP and access control
# ---------------------------------------------------------------------------

variable "allowed_iap_user_email" {
  description = "Single Google account email allowed to access the app (e.g. you@gmail.com). No allUsers or allAuthenticatedUsers."
  type        = string
}

variable "iap_audience" {
  description = "IAP JWT audience for backend validation. After first apply, run: terraform output -raw backend_service_audience and set this variable, then apply again so the backend can validate IAP JWTs. Leave empty to skip IAP validation in the backend (not recommended for production)."
  type        = string
  default     = ""
}

# ---------------------------------------------------------------------------
# Load balancer / HTTPS
# ---------------------------------------------------------------------------

variable "frontend_domain" {
  description = "Domain for the IAP-protected frontend (e.g. album.example.com). Used for managed SSL certificate; you must create a DNS A record pointing to the LB IP after apply."
  type        = string
}
