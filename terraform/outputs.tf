output "bucket_name" {
  description = "Managed media bucket name."
  value       = google_storage_bucket.media.name
}

output "service_name" {
  description = "Backend Cloud Run service name."
  value       = google_cloud_run_v2_service.backend.name
}

output "cloud_run_url" {
  description = "Cloud Run service URL."
  value       = google_cloud_run_v2_service.backend.uri
}

output "firestore_database_name" {
  description = "Firestore database used for photo metadata."
  value       = google_firestore_database.default.name
}
