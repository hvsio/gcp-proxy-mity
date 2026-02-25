output "cloud_run_url" {
  description = "URL of the Cloud Run service"
  value       = google_cloud_run_v2_service.app.uri
}

output "bucket_name" {
  description = "Name of the storage bucket"
  value       = google_storage_bucket.storage.name
}

output "cloudsql_ip" {
  description = "Private IP of the Cloud SQL instance"
  value       = google_sql_database_instance.postgres.private_ip_address
  sensitive   = true
}
