# Terraform

GCP infrastructure for the Uni Album backend:

- Cloud Run service for `gcp-proxy-mity`
- Existing Google Cloud Storage bucket, defaulting to `aj-cloud`
- Firestore Native database and composite indexes for album, all-assets, favorites, and exact-tag collection reads
- Optional backend IAP JWT validation through service environment variables

## Files

```text
backend.tf      # Terraform state backend declaration
main.tf         # GCP APIs, Firestore indexes, bucket IAM, Cloud Run
variables.tf    # project, region, image, IAP, CORS variables
outputs.tf      # Cloud Run URL, bucket name, optional Cloud SQL connection name
```

## Required Variables

| Variable | Description |
| --- | --- |
| `project_id` | GCP project ID |
| `region` | GCP region, defaults to `europe-west4` |
| `container_image` | Immutable Cloud Run image, not `:latest` |
| `storage_bucket_name` | Existing GCS bucket used by the app, defaults to `aj-cloud` |
| `enable_database` | Creates Cloud SQL and enables app DB startup when `true` |
| `vpc_connector_id` | Optional existing connector for private-ranges-only egress |
| `allow_public_invoker` | Grants unauthenticated Cloud Run invocation when `true` |
| `allowed_iap_user_emails` | Emails allowed by backend IAP validation |
| `iap_audience` | Expected IAP JWT audience; empty disables backend JWT validation |
| `cors_allowed_origins` | List of allowed CORS origins |

## Low-Cost Database Defaults

When `enable_database = true`, the database defaults favor the lowest-cost managed PostgreSQL profile:

| Variable | Default | Cost effect |
| --- | --- | --- |
| `db_disk_type` | `PD_HDD` | Uses cheaper persistent disk storage for low-traffic workloads. |
| `db_point_in_time_recovery_enabled` | `false` | Avoids PITR storage overhead. |
| `db_backup_retention_count` | `1` | Keeps only the latest automated backup. |
| `db_activation_policy` | `ALWAYS` | Keeps the DB available; set `NEVER` only when planned downtime is acceptable. |
| `db_max_connections` | `3` | Keeps each Cloud Run instance from opening a large idle pool against `db-f1-micro`. |

With one retained backup and no PITR, recovery is limited to the latest automated backup rather than an arbitrary point in time.

## Apply

```bash
terraform init \
  -backend-config=bucket=YOUR_TERRAFORM_STATE_BUCKET \
  -backend-config=prefix=gcp-proxy-mity
terraform plan
terraform apply
```

Cloud Run depends on the Firestore composite indexes for `favorite + uploadedAt + id` and `tags array-contains + uploadedAt + id`, so Terraform creates those indexes before the backend revision receives traffic.

## Outputs

```bash
terraform output -raw cloud_run_url
terraform output -raw bucket_name
terraform output -raw cloudsql_connection_name
```
