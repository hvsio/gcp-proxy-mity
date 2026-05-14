output "cloud_run_url" {
  description = "URL of the Cloud Run service"
  value       = google_cloud_run_v2_service.app.uri
}

output "bucket_name" {
  description = "Name of the storage bucket"
  value       = var.storage_bucket_name
}

output "cloudsql_connection_name" {
  description = "Cloud SQL instance connection name"
  value       = var.enable_database ? google_sql_database_instance.app[0].connection_name : null
}
