# Terraform Backend Configuration
# This stores Terraform state in Google Cloud Storage

terraform {
  backend "gcs" {
    bucket = "homey-bw58-terraform-state"
    prefix = "gcp-proxy-mity"
  }
}

# Create the state bucket (run this once manually first)
# resource "google_storage_bucket" "terraform_state" {
#   name          = "${var.project_id}-terraform-state"
#   location      = "EU"
#   force_destroy = true
#   
#   uniform_bucket_level_access = true
#   
#   versioning {
#     enabled = true
#   }
#   
#   lifecycle_rule {
#     condition {
#       age = 90
#     }
#     action {
#       type = "Delete"
#     }
#   }
# }
