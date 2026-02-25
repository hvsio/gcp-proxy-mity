# ---------------------------------------------------------------------------
# GCP Proxy Mity – Backend infrastructure
# Cloud Run + Cloud SQL + Private Networking
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
  for_each = toset([
    "compute.googleapis.com",
    "run.googleapis.com",
    "vpcaccess.googleapis.com",
    "sqladmin.googleapis.com",
    "storage.googleapis.com",
    "servicenetworking.googleapis.com",
    "secretmanager.googleapis.com",
    "artifactregistry.googleapis.com",
  ])
  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

# ---------------------------------------------------------------------------
# VPC and networking
# ---------------------------------------------------------------------------

resource "google_compute_network" "vpc" {
  name                    = "gcp-proxy-mity-vpc"
  auto_create_subnetworks = false
  depends_on              = [google_project_service.apis]
}

resource "google_compute_subnetwork" "subnet" {
  name                     = "gcp-proxy-mity-subnet"
  ip_cidr_range            = "10.1.0.0/24"
  region                   = var.region
  network                  = google_compute_network.vpc.id
  private_ip_google_access = true
}

resource "google_compute_global_address" "private_ip_range" {
  name          = "gcp-proxy-mity-private-ip"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.vpc.id
}

resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_range.name]
  depends_on              = [google_project_service.apis]
}

resource "google_vpc_access_connector" "connector" {
  provider      = google-beta
  name          = "gcp-proxy-mity-connector"
  region        = var.region
  network       = google_compute_network.vpc.name
  ip_cidr_range = "10.2.0.0/28"
  depends_on    = [google_project_service.apis, google_compute_subnetwork.subnet]
}

# ---------------------------------------------------------------------------
# Cloud SQL PostgreSQL
# ---------------------------------------------------------------------------

resource "random_password" "db_password" {
  length  = 32
  special = false
}

resource "google_secret_manager_secret" "db_password" {
  secret_id = "cloudsql-password"
  replication {
    auto {}
  }
  depends_on = [google_project_service.apis]
}

resource "google_secret_manager_secret_version" "db_password" {
  secret      = google_secret_manager_secret.db_password.id
  secret_data = random_password.db_password.result
}

resource "google_sql_database_instance" "postgres" {
  name                = "gcp-proxy-mity-db"
  database_version    = "POSTGRES_15"
  region              = var.region
  deletion_protection = true

  settings {
    tier              = "db-f1-micro"
    availability_type = "ZONAL"
    disk_size         = 10
    disk_type         = "PD_SSD"

    ip_configuration {
      ipv4_enabled                                  = false
      private_network                               = google_compute_network.vpc.id
      enable_private_path_for_google_cloud_services = true
    }

    backup_configuration {
      enabled                        = true
      start_time                     = "02:00"
      point_in_time_recovery_enabled = true
      backup_retention_settings {
        retained_backups = 7
      }
    }
  }

  depends_on = [
    google_project_service.apis,
    google_service_networking_connection.private_vpc_connection,
  ]
}

resource "google_sql_database" "app" {
  name     = "postgres"
  instance = google_sql_database_instance.postgres.name
}

resource "google_sql_user" "postgres" {
  name     = "postgres"
  instance = google_sql_database_instance.postgres.name
  password = random_password.db_password.result
}

# ---------------------------------------------------------------------------
# GCS bucket
# ---------------------------------------------------------------------------

resource "google_storage_bucket" "storage" {
  name                        = "${var.project_id}-gcp-proxy-mity-storage"
  location                    = var.region
  uniform_bucket_level_access = true
  versioning { enabled = false }
  lifecycle_rule {
    condition { age = 90 }
    action { type = "Delete" }
  }
  depends_on = [google_project_service.apis]
}

# ---------------------------------------------------------------------------
# Service account and IAM
# ---------------------------------------------------------------------------

resource "google_service_account" "app_sa" {
  account_id   = "gcp-proxy-mity-app"
  display_name = "GCP Proxy Mity App Service Account"
  depends_on   = [google_project_service.apis]
}

resource "google_storage_bucket_iam_member" "storage_admin" {
  bucket = google_storage_bucket.storage.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.app_sa.email}"
}

resource "google_project_iam_member" "cloudsql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.app_sa.email}"
}

resource "google_secret_manager_secret_iam_member" "app_sa_secret_access" {
  secret_id = google_secret_manager_secret.db_password.secret_id
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

    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "ALL_TRAFFIC"
    }

    containers {
      image = var.container_image != "" ? var.container_image : "${var.region}-docker.pkg.dev/${var.project_id}/gcp-proxy-mity/gcp-proxy-mity:latest"

      ports {
        container_port = 8080
      }

      env {
        name  = "GCP_PROJECT_ID"
        value = var.project_id
      }
      env {
        name  = "GCS_BUCKET_NAME"
        value = google_storage_bucket.storage.name
      }
      env {
        name  = "DB_TYPE"
        value = "postgres"
      }
      env {
        name  = "DB_HOST"
        value = google_sql_database_instance.postgres.private_ip_address
      }
      env {
        name  = "DB_PORT"
        value = "5432"
      }
      env {
        name  = "DB_DATABASE_NAME"
        value = "postgres"
      }
      env {
        name  = "DB_USERNAME"
        value = "postgres"
      }
      env {
        name  = "DB_SSL_MODE"
        value = "require"
      }
      env {
        name = "DB_PASSWORD"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.db_password.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "IAP_AUDIENCE"
        value = var.iap_audience
      }
      env {
        name  = "ALLOWED_IAP_EMAILS"
        value = var.allowed_iap_user_email
      }
      env {
        name  = "CORS_ALLOWED_ORIGINS"
        value = var.cors_allowed_origins
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
    google_vpc_access_connector.connector,
    google_sql_database_instance.postgres,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.app.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}


