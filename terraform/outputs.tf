# ---------------------------------------------------------------------------
# Outputs
# ---------------------------------------------------------------------------

output "iap_protected_url" {
  description = "IAP-protected frontend URL (HTTPS). Only the allowed Google account can access."
  value       = "https://${var.frontend_domain}"
}

output "load_balancer_ip" {
  description = "External IP of the HTTPS Load Balancer. Point your DNS A record for frontend_domain to this IP."
  value       = google_compute_global_address.lb_ip.address
}

output "frontend_service_account_email" {
  description = "Frontend Cloud Run service account (invokes backend only)."
  value       = google_service_account.frontend_sa.email
}

output "backend_service_account_email" {
  description = "Backend Cloud Run service account (GCS, Cloud SQL, secrets)."
  value       = google_service_account.backend_sa.email
}

output "backend_cloud_run_uri" {
  description = "Backend Cloud Run URI (internal only; used by frontend)."
  value       = google_cloud_run_v2_service.backend.uri
  sensitive   = true
}

output "backend_service_audience" {
  description = "Set this as iap_audience in tfvars and re-apply so the backend validates IAP JWTs: /projects/PROJECT_NUMBER/global/backendServices/BACKEND_SERVICE_ID"
  value       = "/projects/${data.google_project.project.number}/global/backendServices/${google_compute_backend_service.frontend.id}"
}

output "bucket_name" {
  description = "GCS bucket name for backend storage."
  value       = google_storage_bucket.storage.name
}

output "verification_commands" {
  description = "Commands to verify the setup."
  value       = <<-EOT
    # 1. Frontend inaccessible without Google login (expect 403 or IAP challenge):
    curl -sI https://${var.frontend_domain}

    # 2. Backend not callable directly (expect 403; backend has internal ingress only):
    curl -sI ${google_cloud_run_v2_service.backend.uri}/health

    # 3. Only allowed account: sign in with ${var.allowed_iap_user_email} in a browser (including iPhone/iPad) and open:
    open https://${var.frontend_domain}

    # 4. DNS: ensure an A record for ${var.frontend_domain} points to: ${google_compute_global_address.lb_ip.address}
  EOT
}
