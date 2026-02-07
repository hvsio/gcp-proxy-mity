# GCP Proxy Mity Infrastructure
# Cloud Run + AlloyDB Omni + Cloud IAP + Private Networking

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

# Variables
variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "europe-west4"
}

variable "domain_name" {
  description = "Custom domain for the application (optional)"
  type        = string
  default     = ""
}

variable "oauth_client_id" {
  description = "OAuth 2.0 client ID for IAP"
  type        = string
}

variable "oauth_client_secret" {
  description = "OAuth 2.0 client secret for IAP"
  type        = string
  sensitive   = true
}

variable "allowed_users" {
  description = "List of users allowed to access via IAP (email addresses)"
  type        = list(string)
}

variable "container_image" {
  description = "Container image to deploy"
  type        = string
  default     = ""
}

# Provider configuration
provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

# Enable required APIs
resource "google_project_service" "apis" {
  for_each = toset([
    "run.googleapis.com",
    "vpcaccess.googleapis.com",
    "alloydb.googleapis.com",
    "storage.googleapis.com",
    "compute.googleapis.com",
    "iap.googleapis.com",
    "certificatemanager.googleapis.com",
    "servicenetworking.googleapis.com",
    "secretmanager.googleapis.com"
  ])
  
  project = var.project_id
  service = each.value
  
  disable_on_destroy = false
}

# VPC and Networking
resource "google_compute_network" "vpc" {
  name                    = "gcp-proxy-mity-vpc"
  auto_create_subnetworks = false
  
  depends_on = [google_project_service.apis]
}

resource "google_compute_subnetwork" "subnet" {
  name          = "gcp-proxy-mity-subnet"
  ip_cidr_range = "10.1.0.0/24"
  region        = var.region
  network       = google_compute_network.vpc.id
  
  private_ip_google_access = true
}

# Private Services Access for AlloyDB
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

  depends_on = [google_project_service.apis]
}

# Serverless VPC Connector for Cloud Run
resource "google_vpc_access_connector" "connector" {
  provider = google-beta
  
  name          = "gcp-proxy-mity-connector"
  region        = var.region
  network       = google_compute_network.vpc.name
  ip_cidr_range = "10.2.0.0/28"
  
  depends_on = [
    google_project_service.apis,
    google_compute_subnetwork.subnet
  ]
}

# AlloyDB Omni Cluster
resource "google_alloydb_cluster" "cluster" {
  provider = google-beta
  
  cluster_id = "gcp-proxy-mity-cluster"
  location   = var.region
  
  cluster_type = "PRIMARY"
  
  initial_user {
    user     = "postgres"
    password = random_password.db_password.result
  }

  network_config {
    network = google_compute_network.vpc.id
  }
  
  automated_backup_policy {
    enabled = true
    backup_window = "7200s"
    location = var.region
    
    weekly_schedule {
      days_of_week = ["MONDAY", "WEDNESDAY", "FRIDAY"]
      start_times {
        hours   = 2
        minutes = 0
      }
    }
  }
  
  depends_on = [
    google_project_service.apis,
    google_compute_network.vpc,
    google_service_networking_connection.private_vpc_connection
  ]
}

# AlloyDB Omni Instance
resource "google_alloydb_instance" "instance" {
  provider = google-beta
  
  cluster       = google_alloydb_cluster.cluster.name
  instance_id   = "gcp-proxy-mity-instance"
  instance_type = "PRIMARY"
  
  machine_config {
    cpu_count = 2
  }
  
  depends_on = [google_alloydb_cluster.cluster]
}

# Cloud Storage Bucket
resource "google_storage_bucket" "storage" {
  name     = "${var.project_id}-gcp-proxy-mity-storage"
  location = var.region
  
  uniform_bucket_level_access = true
  
  versioning {
    enabled = false
  }
  
  lifecycle_rule {
    condition {
      age = 90
    }
    action {
      type = "Delete"
    }
  }
  
  depends_on = [google_project_service.apis]
}

# Service Account for Cloud Run
resource "google_service_account" "app_sa" {
  account_id   = "gcp-proxy-mity-app"
  display_name = "GCP Proxy Mity App Service Account"
  
  depends_on = [google_project_service.apis]
}

# IAM bindings for service account
resource "google_storage_bucket_iam_member" "storage_admin" {
  bucket = google_storage_bucket.storage.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.app_sa.email}"
}

resource "google_project_iam_member" "alloydb_client" {
  project = var.project_id
  role    = "roles/alloydb.client"
  member  = "serviceAccount:${google_service_account.app_sa.email}"
}

import {
  to = google_cloud_run_v2_service.app
  id = "projects/${var.project_id}/locations/${var.region}/services/gcp-proxy-mity"
}

# Cloud Run Service
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
        value = google_alloydb_instance.instance.ip_address
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
    google_alloydb_instance.instance
  ]
}

# Secret for database password
resource "google_secret_manager_secret" "db_password" {
  secret_id = "alloydb-password"
  
  replication {
    auto {}
  }
  
  depends_on = [google_project_service.apis]
}

resource "random_password" "db_password" {
  length  = 32
  special = false
}

resource "google_secret_manager_secret_version" "db_password" {
  secret      = google_secret_manager_secret.db_password.id
  secret_data = random_password.db_password.result
}

# IAM for accessing secrets
resource "google_secret_manager_secret_iam_member" "app_sa_secret_access" {
  secret_id = google_secret_manager_secret.db_password.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.app_sa.email}"
}

# Application Load Balancer
resource "google_compute_global_address" "default" {
  name = "gcp-proxy-mity-ip"
  
  depends_on = [google_project_service.apis]
}

# Backend service for Cloud Run
resource "google_compute_region_network_endpoint_group" "cloudrun_neg" {
  name                  = "gcp-proxy-mity-neg"
  network_endpoint_type = "SERVERLESS"
  region                = var.region
  
  cloud_run {
    service = google_cloud_run_v2_service.app.name
  }
  
  depends_on = [google_cloud_run_v2_service.app]
}

resource "google_compute_backend_service" "default" {
  name        = "gcp-proxy-mity-backend"
  protocol    = "HTTP"
  timeout_sec = 30
  
  backend {
    group = google_compute_region_network_endpoint_group.cloudrun_neg.id
  }
  
  # Enable IAP
  iap {
    oauth2_client_id     = var.oauth_client_id
    oauth2_client_secret = var.oauth_client_secret
  }
  
  depends_on = [google_project_service.apis]
}

# URL Map
resource "google_compute_url_map" "default" {
  name            = "gcp-proxy-mity-urlmap"
  default_service = google_compute_backend_service.default.id
}

# SSL Certificate (managed)
resource "google_compute_managed_ssl_certificate" "default" {
  count = var.domain_name != "" ? 1 : 0
  
  name = "gcp-proxy-mity-ssl"
  
  managed {
    domains = [var.domain_name]
  }
  
  depends_on = [google_project_service.apis]
}

# HTTPS Proxy
resource "google_compute_target_https_proxy" "default" {
  name   = "gcp-proxy-mity-https-proxy"
  url_map = google_compute_url_map.default.id
  
  ssl_certificates = var.domain_name != "" ? [google_compute_managed_ssl_certificate.default[0].id] : []
  
  depends_on = [google_project_service.apis]
}

# Forwarding Rule
resource "google_compute_global_forwarding_rule" "default" {
  name       = "gcp-proxy-mity-forwarding-rule"
  target     = google_compute_target_https_proxy.default.id
  port_range = "443"
  ip_address = google_compute_global_address.default.id
}

# IAM binding for IAP access
resource "google_iap_web_backend_service_iam_binding" "binding" {
  web_backend_service = google_compute_backend_service.default.name
  role                = "roles/iap.httpsResourceAccessor"
  members             = [for user in var.allowed_users : "user:${user}"]
}

# Outputs
output "load_balancer_ip" {
  description = "IP address of the load balancer"
  value       = google_compute_global_address.default.address
}

output "cloud_run_url" {
  description = "URL of the Cloud Run service (internal)"
  value       = google_cloud_run_v2_service.app.uri
}

output "bucket_name" {
  description = "Name of the storage bucket"
  value       = google_storage_bucket.storage.name
}

output "alloydb_ip" {
  description = "Private IP of the AlloyDB instance"
  value       = google_alloydb_instance.instance.ip_address
  sensitive   = true
}
