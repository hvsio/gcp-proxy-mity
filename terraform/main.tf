terraform {
  required_version = ">= 1.7"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 5.0"
    }
  }
}

locals {
  project_services = toset([
    "artifactregistry.googleapis.com",
    "firestore.googleapis.com",
    "firebase.googleapis.com",
    "iam.googleapis.com",
    "iamcredentials.googleapis.com",
    "run.googleapis.com",
    "storage.googleapis.com",
  ])
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

resource "google_project_service" "apis" {
  for_each = local.project_services

  project            = var.project_id
  service            = each.value
  disable_on_destroy = false
}

resource "google_storage_bucket" "media" {
  name                        = var.storage_bucket_name
  location                    = "EUR4"
  storage_class               = "STANDARD"
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  rpo                         = "ASYNC_TURBO"

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.apis]
}

resource "google_firebase_project" "firebase" {
  provider = google-beta
  project  = var.project_id

  lifecycle {
    prevent_destroy = true
  }

  depends_on = [google_project_service.apis]
}

resource "google_firestore_database" "default" {
  project                 = var.project_id
  name                    = "(default)"
  location_id             = var.region
  type                    = "FIRESTORE_NATIVE"
  delete_protection_state = "DELETE_PROTECTION_ENABLED"
  deletion_policy         = "ABANDON"

  depends_on = [
    google_project_service.apis,
    google_firebase_project.firebase,
  ]
}

resource "google_firestore_index" "album_assets_by_uploaded_at" {
  project     = var.project_id
  database    = google_firestore_database.default.name
  collection  = "photo_album_assets"
  query_scope = "COLLECTION"

  fields {
    field_path = "albumId"
    order      = "ASCENDING"
  }

  fields {
    field_path = "assetUploadedAt"
    order      = "DESCENDING"
  }

  fields {
    field_path = "assetId"
    order      = "DESCENDING"
  }
}

resource "google_firestore_index" "assets_by_uploaded_at" {
  project     = var.project_id
  database    = google_firestore_database.default.name
  collection  = "photo_assets"
  query_scope = "COLLECTION"

  fields {
    field_path = "uploadedAt"
    order      = "DESCENDING"
  }

  fields {
    field_path = "id"
    order      = "DESCENDING"
  }
}

resource "google_firestore_index" "albums_by_created_at" {
  project     = var.project_id
  database    = google_firestore_database.default.name
  collection  = "photo_albums"
  query_scope = "COLLECTION"

  fields {
    field_path = "createdAt"
    order      = "ASCENDING"
  }

  fields {
    field_path = "id"
    order      = "ASCENDING"
  }
}

resource "google_firestore_index" "jobs_by_created_at" {
  project     = var.project_id
  database    = google_firestore_database.default.name
  collection  = "photo_jobs"
  query_scope = "COLLECTION"

  fields {
    field_path = "createdAt"
    order      = "DESCENDING"
  }

  fields {
    field_path = "id"
    order      = "DESCENDING"
  }
}

resource "google_service_account" "backend" {
  account_id   = "gcp-proxy-mity-backend"
  display_name = "GCP Proxy Mity Backend"

  depends_on = [google_project_service.apis]
}

resource "google_storage_bucket_iam_member" "backend_storage_object_creator" {
  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectCreator"
  member = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_storage_bucket_iam_member" "backend_storage_object_viewer" {
  bucket = google_storage_bucket.media.name
  role   = "roles/storage.objectViewer"
  member = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_project_iam_member" "backend_firestore_user" {
  project = var.project_id
  role    = "roles/datastore.user"
  member  = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_service_account_iam_member" "backend_token_creator" {
  service_account_id = google_service_account.backend.name
  role               = "roles/iam.serviceAccountTokenCreator"
  member             = "serviceAccount:${google_service_account.backend.email}"
}

resource "google_cloud_run_v2_service" "backend" {
  provider = google-beta
  name     = var.service_name
  location = var.region

  template {
    service_account = google_service_account.backend.email

    containers {
      image = var.container_image

      ports {
        container_port = 8080
      }

      env {
        name  = "ENABLE_DATABASE"
        value = "true"
      }

      env {
        name  = "PHOTO_METADATA_BACKEND"
        value = "firestore"
      }

      env {
        name  = "FIRESTORE_DATABASE"
        value = google_firestore_database.default.name
      }

      env {
        name  = "GCP_PROJECT_ID"
        value = var.project_id
      }

      env {
        name  = "GCS_BUCKET_NAME"
        value = google_storage_bucket.media.name
      }

      env {
        name  = "SIGNED_URL_SERVICE_ACCOUNT_EMAIL"
        value = google_service_account.backend.email
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
          path = "/ready"
          port = 8080
        }
        initial_delay_seconds = 5
        period_seconds        = 5
        failure_threshold     = 12
        timeout_seconds       = 3
      }

      liveness_probe {
        http_get {
          path = "/health"
          port = 8080
        }
        initial_delay_seconds = 10
        period_seconds        = 10
        failure_threshold     = 3
        timeout_seconds       = 3
      }
    }

    scaling {
      min_instance_count = 0
      max_instance_count = 10
    }
  }

  depends_on = [
    google_firestore_database.default,
    google_firestore_index.album_assets_by_uploaded_at,
    google_firestore_index.albums_by_created_at,
    google_firestore_index.assets_by_uploaded_at,
    google_firestore_index.jobs_by_created_at,
    google_project_iam_member.backend_firestore_user,
    google_service_account_iam_member.backend_token_creator,
    google_storage_bucket_iam_member.backend_storage_object_creator,
    google_storage_bucket_iam_member.backend_storage_object_viewer,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "frontend_invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.backend.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${var.frontend_service_account_email}"
}

resource "google_cloud_run_v2_service_iam_member" "deployment_invoker" {
  project  = var.project_id
  location = var.region
  name     = google_cloud_run_v2_service.backend.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${var.deployment_service_account_email}"
}
