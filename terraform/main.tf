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

removed {
  from = google_cloud_run_v2_service.app

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_cloud_run_v2_service_iam_member.public

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_service_account.app_sa

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_storage_bucket_iam_member.storage_viewer

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_project_iam_member.app_cloudsql_client

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_secret_manager_secret_iam_member.app_sa_secret_access_app_db_password

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_sql_database_instance.app

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_sql_database.app_database

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_sql_user.app

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_secret_manager_secret.app_db_password

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_secret_manager_secret_version.app_db_password

  lifecycle {
    destroy = true
  }
}

removed {
  from = random_password.app_db_password

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_project_service.apis

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_compute_network.vpc

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_compute_subnetwork.subnet

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_compute_global_address.private_ip_range

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_service_networking_connection.private_vpc_connection

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_vpc_access_connector.connector

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_storage_bucket.storage

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_storage_bucket_iam_member.storage_admin

  lifecycle {
    destroy = true
  }
}

removed {
  from = random_password.db_password

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_secret_manager_secret.db_password

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_secret_manager_secret_version.db_password

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_sql_database_instance.postgres

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_sql_database.app

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_sql_user.postgres

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_project_iam_member.cloudsql_client

  lifecycle {
    destroy = true
  }
}

removed {
  from = google_secret_manager_secret_iam_member.app_sa_secret_access

  lifecycle {
    destroy = true
  }
}
