# Terraform

GCP infrastructure for the read-only storage proxy:

- Cloud Run service for `gcp-proxy-mity`
- Google Cloud Storage bucket
- Cloud SQL Postgres instance kept for future metadata features
- VPC connector and private service networking
- Secret Manager entry for the database password
- Optional backend IAP JWT validation through service environment variables

## Files

```text
backend.tf      # Terraform state backend
main.tf         # GCP APIs, networking, Cloud SQL, bucket, Cloud Run, IAM
variables.tf    # project, region, image, IAP, CORS variables
outputs.tf      # Cloud Run URL, bucket name, Cloud SQL private IP
```

## Required Variables

| Variable | Description |
| --- | --- |
| `project_id` | GCP project ID |
| `region` | GCP region, defaults to `europe-west4` |
| `container_image` | Optional Cloud Run image override |
| `allowed_iap_user_email` | Email allowed by backend IAP validation |
| `iap_audience` | Expected IAP JWT audience; empty disables backend JWT validation |
| `cors_allowed_origins` | Comma-separated allowed CORS origins |

## Apply

```bash
terraform init
terraform plan
terraform apply
```

## Outputs

```bash
terraform output -raw cloud_run_url
terraform output -raw bucket_name
terraform output -raw cloudsql_ip
```
