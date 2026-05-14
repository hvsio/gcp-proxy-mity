# Terraform

GCP infrastructure for the read-only storage proxy:

- Cloud Run service for `gcp-proxy-mity`
- Google Cloud Storage bucket
- Optional Cloud SQL Postgres instance for metadata features
- Optional existing VPC connector with private-ranges-only egress
- Secret Manager entry for the database password when Cloud SQL is enabled
- Optional backend IAP JWT validation through service environment variables

## Files

```text
backend.tf      # Terraform state backend declaration
main.tf         # GCP APIs, optional Cloud SQL, bucket, Cloud Run, IAM
variables.tf    # project, region, image, IAP, CORS variables
outputs.tf      # Cloud Run URL, bucket name, optional Cloud SQL connection name
```

## Required Variables

| Variable | Description |
| --- | --- |
| `project_id` | GCP project ID |
| `region` | GCP region, defaults to `europe-west4` |
| `container_image` | Immutable Cloud Run image, not `:latest` |
| `enable_database` | Creates Cloud SQL and enables app DB startup when `true` |
| `vpc_connector_id` | Optional existing connector for private-ranges-only egress |
| `allow_public_invoker` | Grants unauthenticated Cloud Run invocation when `true` |
| `allowed_iap_user_emails` | Emails allowed by backend IAP validation |
| `iap_audience` | Expected IAP JWT audience; empty disables backend JWT validation |
| `cors_allowed_origins` | List of allowed CORS origins |
| `bucket_lifecycle_delete_age_days` | Optional object deletion age; `null` keeps objects indefinitely |

## Apply

```bash
terraform init \
  -backend-config=bucket=YOUR_TERRAFORM_STATE_BUCKET \
  -backend-config=prefix=gcp-proxy-mity
terraform plan
terraform apply
```

## Outputs

```bash
terraform output -raw cloud_run_url
terraform output -raw bucket_name
terraform output -raw cloudsql_connection_name
```
