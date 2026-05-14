# ---------------------------------------------------------------------------
# GCP Proxy Mity - Backend infrastructure
# Cloud Run + GCS, with optional Cloud SQL
# ---------------------------------------------------------------------------

terraform {
  required_version = ">= 1.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

locals {
  base_project_services = [
    "run.googleapis.com",
    "storage.googleapis.com",
    "artifactregistry.googleapis.com",
  ]

  database_project_services = var.enable_database ? [
    "sqladmin.googleapis.com",
    "secretmanager.googleapis.com",
  ] : []

  vpc_project_services = var.vpc_connector_id != "" ? [
    "vpcaccess.googleapis.com",
  ] : []

  project_services = toset(concat(
    local.base_project_services,
    local.database_project_services,
    local.vpc_project_services,
  ))
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

# ---------------------------------------------------------------------------
# APIs
# ---------------------------------------------------------------------------

resource "google_project_service" "apis" {
  for_each = local.project_services
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# ---------------------------------------------------------------------------
# Cloud SQL PostgreSQL
# ---------------------------------------------------------------------------

resource "random_password" "db_password" {
  count   = var.enable_database ? 1 : 0
  length  = 32
  special = false
}

resource "google_secret_manager_secret" "db_password" {
  count     = var.enable_database ? 1 : 0
  secret_id = "gcp-proxy-mity-cloudsql-password"
  replication {
    auto {}
  }
  depends_on = [google_project_service.apis]
}

resource "google_secret_manager_secret_version" "db_password" {
  count       = var.enable_database ? 1 : 0
  secret      = google_secret_manager_secret.db_password[0].id
  secret_data = random_password.db_password[0].result
}

resource "google_sql_database_instance" "postgres" {
  count               = var.enable_database ? 1 : 0
  name                = "gcp-proxy-mity-db"
  database_version    = "POSTGRES_15"
  region              = var.region
  deletion_protection = true

  settings {
    activation_policy = var.db_activation_policy
    tier              = "db-f1-micro"
    availability_type = "ZONAL"
    disk_size         = 10
    disk_type         = var.db_disk_type

    backup_configuration {
      enabled                        = true
      start_time                     = "02:00"
      point_in_time_recovery_enabled = var.db_point_in_time_recovery_enabled
      backup_retention_settings {
        retained_backups = var.db_backup_retention_count
      }
    }
  }

  depends_on = [
    google_project_service.apis,
  ]
}

resource "google_sql_database" "app" {
  count    = var.enable_database ? 1 : 0
  name     = "gcp_proxy"
  instance = google_sql_database_instance.postgres[0].name
}

resource "google_sql_user" "app" {
  count    = var.enable_database ? 1 : 0
  name     = "gcp_proxy_app"
  instance = google_sql_database_instance.postgres[0].name
  password = random_password.db_password[0].result
}

# ---------------------------------------------------------------------------
# Service account and IAM
# ---------------------------------------------------------------------------

resource "google_service_account" "app_sa" {
  account_id   = "gcp-proxy-mity-app"
  display_name = "GCP Proxy Mity App Service Account"
  depends_on   = [google_project_service.apis]
}

resource "google_storage_bucket_iam_member" "storage_viewer" {
  bucket = var.storage_bucket_name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.app_sa.email}"
}

resource "google_project_iam_member" "cloudsql_client" {
  count   = var.enable_database ? 1 : 0
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.app_sa.email}"
}

resource "google_secret_manager_secret_iam_member" "app_sa_secret_access" {
  count     = var.enable_database ? 1 : 0
  secret_id = google_secret_manager_secret.db_password[0].secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.app_sa.email}"
}

# ---------------------------------------------------------------------------
# Cloud Run – Backend
# ---------------------------------------------------------------------------

resource "google_cloud_run_v2_service" "app" {
  provider = google-beta
  name     = "gcp-proxy-mity"
  location = var.region

  template {
    service_account = google_service_account.app_sa.email

    dynamic "vpc_access" {
      for_each = var.vpc_connector_id == "" ? [] : [var.vpc_connector_id]

      content {
        connector = vpc_access.value
        egress    = "PRIVATE_RANGES_ONLY"
      }
    }

    containers {
      image = var.container_image

      ports {
        container_port = 8080
      }

      env {
        name  = "GCP_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "GCS_BUCKET_NAME"
        value = var.storage_bucket_name
      }
      env {
        name  = "ENABLE_DATABASE"
        value = tostring(var.enable_database)
      }

      dynamic "env" {
        for_each = var.enable_database ? [1] : []

        content {
          name  = "DB_TYPE"
          value = "cloudsql"
        }
      }

      dynamic "env" {
        for_each = var.enable_database ? [1] : []

        content {
          name  = "DB_INSTANCE_CONNECTION_NAME"
          value = google_sql_database_instance.postgres[0].connection_name
        }
      }

      dynamic "env" {
        for_each = var.enable_database ? [1] : []

        content {
          name  = "DB_DATABASE_NAME"
          value = google_sql_database.app[0].name
        }
      }

      dynamic "env" {
        for_each = var.enable_database ? [1] : []

        content {
          name  = "DB_USERNAME"
          value = google_sql_user.app[0].name
        }
      }

      dynamic "env" {
        for_each = var.enable_database ? [1] : []

        content {
          name  = "DB_MAX_CONNECTIONS"
          value = tostring(var.db_max_connections)
        }
      }

      dynamic "env" {
        for_each = var.enable_database ? [1] : []

        content {
          name = "DB_PASSWORD"
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.db_password[0].secret_id
              version = "latest"
            }
          }
        }
      }
      env {
        name  = "IAP_AUDIENCE"
        value = var.iap_audience
      }
      env {
        name  = "ALLOWED_IAP_EMAILS"
        value = join(",", var.allowed_iap_user_emails)
      }
      env {
        name  = "CORS_ALLOWED_ORIGINS"
        value = join(",", var.cors_allowed_origins)
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "1Gi"
        }
      }

      startup_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        initial_delay_seconds = 5
        period_seconds        = 5
        failure_threshold     = 12
        timeout_seconds       = 3
      }
    }

    scaling {
      min_instance_count = 0
      max_instance_count = 10
    }
  }

  depends_on = [
    google_project_service.apis,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  count    = var.allow_public_invoker ? 1 : 0
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.app.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
