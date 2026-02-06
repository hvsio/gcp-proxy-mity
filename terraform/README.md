# GCP Proxy Mity - Terraform Infrastructure

This Terraform configuration sets up a secure, private Cloud Run application with AlloyDB Omni database and Cloud Identity-Aware Proxy (IAP) authentication.

## Architecture

- **Cloud Run**: Private service (no public access)
- **AlloyDB Omni**: Managed PostgreSQL database in private VPC
- **Application Load Balancer**: Public endpoint with HTTPS
- **Cloud IAP**: Authentication layer (your Google account)
- **Cloud Storage**: File storage bucket
- **VPC**: Private networking with serverless connector

## Prerequisites

1. **GCP Project** with billing enabled
2. **OAuth 2.0 Credentials** for IAP:
   - Go to [Google Cloud Console > APIs & Credentials](https://console.cloud.google.com/apis/credentials)
   - Create OAuth 2.0 Client ID (Web application)
   - Add authorized redirect URIs: `https://iap.googleapis.com/v1/oauth/clientIds/{client-id}:handleRedirect`
3. **Docker image** pushed to Google Container Registry
4. **Domain name** (optional, but recommended)

## Setup Instructions

### 1. Build and Push Docker Image

```bash
# Build the image
docker build -t gcr.io/YOUR-PROJECT-ID/gcp-proxy-mity:latest .

# Push to GCR
docker push gcr.io/YOUR-PROJECT-ID/gcp-proxy-mity:latest
```

### 2. Configure Terraform Variables

```bash
# Copy the example file
cp terraform.tfvars.example terraform.tfvars

# Edit with your values
vim terraform.tfvars
```

Required values:
- `project_id`: Your GCP project ID
- `oauth_client_id`: OAuth client ID from step 2
- `oauth_client_secret`: OAuth client secret from step 2
- `allowed_users`: List of email addresses (your accounts)

### 3. Deploy Infrastructure

```bash
# Initialize Terraform
terraform init

# Review the plan
terraform plan

# Deploy
terraform apply
```

### 4. Configure DNS (if using custom domain)

After deployment, point your domain to the load balancer IP:

```bash
# Get the IP address
terraform output load_balancer_ip
```

Create an A record: `api.yourdomain.com -> LOAD_BALANCER_IP`

### 5. Access Your Application

Once deployed, access your application at:
- Custom domain: `https://api.yourdomain.com`
- Load balancer IP: `https://LOAD_BALANCER_IP` (will show certificate warning)

You'll be prompted to authenticate with your Google account.

## Security Features

✅ **Private Cloud Run** - No direct public access  
✅ **Private Database** - AlloyDB in VPC, not internet accessible  
✅ **IAP Authentication** - Google account required  
✅ **HTTPS Only** - Managed SSL certificates  
✅ **VPC Isolation** - All services in private network  
✅ **Least Privilege IAM** - Minimal service account permissions  

## Mobile Access

On your iPhone/iPad:
1. Open Safari or any browser
2. Navigate to your application URL
3. Authenticate with your Google account
4. Save to home screen for app-like experience

## Cost Optimization

- **Cloud Run**: Scales to zero, pay per request
- **AlloyDB Omni**: Serverless, scales with usage
- **Load Balancer**: Pay per GB of traffic
- **Storage**: Pay per GB stored

## Monitoring

Access logs and metrics in Google Cloud Console:
- **Cloud Run**: Monitor requests, latency, errors
- **AlloyDB**: Database performance metrics
- **Load Balancer**: Traffic and uptime monitoring

## Troubleshooting

### Common Issues

1. **OAuth Error**: Ensure redirect URIs are correctly configured
2. **Database Connection**: Check VPC connector and AlloyDB networking
3. **403 Forbidden**: Verify your email is in `allowed_users` list
4. **SSL Certificate**: May take 15-60 minutes to provision

### Check Service Health

```bash
# Test the health endpoint (after IAP authentication)
curl -H "Authorization: Bearer $(gcloud auth print-identity-token)" \
  https://your-domain.com/health
```

## Clean Up

```bash
# Destroy all resources
terraform destroy
```

**Warning**: This will delete all data including the database and storage bucket.
