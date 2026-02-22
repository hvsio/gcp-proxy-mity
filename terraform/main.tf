# ---------------------------------------------------------------------------
# GCP Proxy Mity + Uni Album – Private-by-default with IAP
# VPC, Backend (internal only), Frontend (LB + IAP only), HTTPS, single-user access
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
# Data
# ---------------------------------------------------------------------------

data "google_project" "project" {
  project_id = var.project_id
}

# ---------------------------------------------------------------------------
# APIs
# ---------------------------------------------------------------------------

resource "google_project_service" "apis" {
  for_each = toset([
    "compute.googleapis.com",
    "run.googleapis.com",
    "iap.googleapis.com",
    "cloudresourcemanager.googleapis.com",
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
# Cloud SQL PostgreSQL (backend DB)
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
# Service accounts – least privilege
# ---------------------------------------------------------------------------

resource "google_service_account" "backend_sa" {
  account_id   = "gcp-proxy-mity-backend"
  display_name = "Backend Cloud Run Service Account"
  depends_on   = [google_project_service.apis]
}

resource "google_service_account" "frontend_sa" {
  account_id   = "uni-album-frontend"
  display_name = "Frontend Cloud Run Service Account"
  depends_on   = [google_project_service.apis]
}

resource "google_storage_bucket_iam_member" "backend_storage" {
  bucket = google_storage_bucket.storage.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.backend_sa.email}"
}

resource "google_project_iam_member" "backend_cloudsql_client" {
  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.backend_sa.email}"
}

resource "google_secret_manager_secret_iam_member" "backend_secret_access" {
  secret_id = google_secret_manager_secret.db_password.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backend_sa.email}"
}

# ---------------------------------------------------------------------------
# Backend Cloud Run – internal ingress only, no public URL
# ---------------------------------------------------------------------------

resource "google_cloud_run_v2_service" "backend" {
  provider = google-beta
  name     = "gcp-proxy-mity"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_INTERNAL_ONLY"

  template {
    service_account = google_service_account.backend_sa.email
    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "ALL_TRAFFIC"
    }
    containers {
      image = var.backend_container_image != "" ? var.backend_container_image : "${var.region}-docker.pkg.dev/${var.project_id}/gcp-proxy-mity/gcp-proxy-mity:latest"
      ports { container_port = 8080 }
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
        value = "https://${var.frontend_domain}"
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

# Backend: only frontend SA may invoke (no allUsers, no allAuthenticatedUsers)
resource "google_cloud_run_v2_service_iam_binding" "backend_invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.backend.name
  role     = "roles/run.invoker"
  members  = ["serviceAccount:${google_service_account.frontend_sa.email}"]
}

# ---------------------------------------------------------------------------
# Artifact Registry for frontend (optional; can use existing repo)
# ---------------------------------------------------------------------------

resource "google_artifact_registry_repository" "frontend" {
  location      = var.region
  repository_id = "uni-album"
  format        = "DOCKER"
  depends_on    = [google_project_service.apis]
}

# ---------------------------------------------------------------------------
# Frontend Cloud Run – traffic only via Load Balancer + IAP (no direct *.run.app)
# ---------------------------------------------------------------------------

resource "google_cloud_run_v2_service" "frontend" {
  provider = google-beta
  name     = "uni-album"
  location = var.region
  ingress  = "INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER"

  template {
    service_account = google_service_account.frontend_sa.email
    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "PRIVATE_RANGES_ONLY"
    }
    containers {
      image = var.frontend_container_image != "" ? var.frontend_container_image : "${var.region}-docker.pkg.dev/${var.project_id}/uni-album/uni-album:latest"
      ports { container_port = 8080 }
      env {
        name  = "BACKEND_URL"
        value = google_cloud_run_v2_service.backend.uri
      }
      resources {
        limits = {
          cpu    = "1"
          memory = "256Mi"
        }
      }
      startup_probe {
        http_get {
          path = "/healthz"
          port = 8080
        }
        initial_delay_seconds = 2
        period_seconds        = 3
        failure_threshold     = 10
        timeout_seconds       = 2
      }
    }
    scaling {
      min_instance_count = 0
      max_instance_count = 5
    }
  }
  depends_on = [
    google_project_service.apis,
    google_artifact_registry_repository.frontend,
    google_vpc_access_connector.connector,
  ]
}

# Frontend: only internal + Cloud Load Balancing (no direct public *.run.app)
# We do NOT grant allUsers; only the load balancer identity may invoke.
resource "google_cloud_run_v2_service_iam_member" "frontend_lb_invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.frontend.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:service-${data.google_project.project.number}@gcp-sa-runapps.iam.gserviceaccount.com"
}

# ---------------------------------------------------------------------------
# HTTPS Load Balancer + IAP
# ---------------------------------------------------------------------------

# Reserve global static IP for the LB
resource "google_compute_global_address" "lb_ip" {
  name = "uni-album-lb-ip"
}

# Managed SSL certificate (provisioning can take up to ~15 min; create DNS A record to this IP)
resource "google_compute_managed_ssl_certificate" "frontend_cert" {
  name = "uni-album-frontend-cert"
  managed {
    domains = [var.frontend_domain]
  }
}

# Serverless NEG for frontend Cloud Run
resource "google_compute_region_network_endpoint_group" "frontend_neg" {
  name                  = "uni-album-frontend-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  cloud_run {
    service = google_cloud_run_v2_service.frontend.name
  }
}

# Backend service for the LB (frontend Cloud Run)
resource "google_compute_backend_service" "frontend" {
  name                  = "uni-album-frontend-backend"
  protocol              = "HTTP"
  port_name             = "http"
  load_balancing_scheme = "EXTERNAL_MANAGED"
  backend {
    group = google_compute_region_network_endpoint_group.frontend_neg.id
  }
  iap {
    oauth2_client_id     = google_iap_client.iap_client.client_id
    oauth2_client_secret = google_iap_client.iap_client.secret
  }
}

# IAP OAuth brand (project-level; create once per project)
resource "google_iap_brand" "project_brand" {
  project           = var.project_id
  support_email     = var.allowed_iap_user_email
  application_title = "Uni Album"
}

# IAP OAuth client for the backend service
resource "google_iap_client" "iap_client" {
  display_name = "Uni Album IAP Client"
  brand        = google_iap_brand.project_brand.name
}

# URL map: all traffic to frontend backend service
resource "google_compute_url_map" "frontend" {
  name            = "uni-album-url-map"
  default_service = google_compute_backend_service.frontend.id
}

# HTTPS proxy
resource "google_compute_target_https_proxy" "frontend" {
  name             = "uni-album-https-proxy"
  url_map          = google_compute_url_map.frontend.id
  ssl_certificates = [google_compute_managed_ssl_certificate.frontend_cert.id]
}

# Global forwarding rule (HTTPS)
resource "google_compute_global_forwarding_rule" "frontend_https" {
  name                  = "uni-album-https-forwarding-rule"
  ip_protocol           = "TCP"
  port_range            = "443"
  target                = google_compute_target_https_proxy.frontend.id
  load_balancing_scheme = "EXTERNAL_MANAGED"
  ip_address            = google_compute_global_address.lb_ip.id
}

# ---------------------------------------------------------------------------
# IAP access – only the single allowed Google account (no allUsers)
# ---------------------------------------------------------------------------

resource "google_iap_web_backend_service_iam_member" "iap_user" {
  project             = var.project_id
  web_backend_service = google_compute_backend_service.frontend.name
  role                = "roles/iap.httpsResourceAccessor"
  member              = "user:${var.allowed_iap_user_email}"
}
