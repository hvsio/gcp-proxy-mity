# Terraform: Private-by-default GCP + IAP

Infrastructure for **gcp-proxy-mity** (backend) and **uni-album** (frontend) with Identity-Aware Proxy. Only a single Google account can access the app; frontend and backend have no public ingress.

## Architecture (ASCII)

```
                    Internet (HTTPS only)
                            │
                            ▼
              ┌─────────────────────────────┐
              │  External HTTPS Load Balancer │
              │  (managed SSL, IAP enabled)   │
              └──────────────┬───────────────┘
                             │
                    IAP (Google OAuth)
                    only allowed user
                             │
                             ▼
              ┌─────────────────────────────┐
              │   Frontend (Cloud Run)       │
              │   uni-album                  │
              │   ingress: LB + internal     │
              │   no *.run.app direct        │
              └──────────────┬───────────────┘
                             │
                   /api/* → backend
                   X-Goog-IAP-JWT-Assertion forwarded
                             │
              VPC (Serverless Connector)
              PRIVATE_RANGES_ONLY
                             │
                             ▼
              ┌─────────────────────────────┐
              │   Backend (Cloud Run)        │
              │   gcp-proxy-mity             │
              │   ingress: INTERNAL only     │
              │   invoker: frontend SA only  │
              └──────────────┬───────────────┘
                             │
              VPC ALL_TRAFFIC (AlloyDB, GCS)
                             │
              ┌──────────────┼──────────────┐
              ▼              ▼               ▼
         AlloyDB          GCS Bucket    Secret Manager
```

## How IAP works here

1. **User** opens `https://<frontend_domain>` in a browser (desktop or mobile).
2. **Load Balancer** terminates HTTPS and sends the request to **IAP**.
3. **IAP** challenges the user with **Google OAuth**. Only the Google account that has `roles/iap.httpsResourceAccessor` on the backend service can pass.
4. After login, IAP adds the header **`X-Goog-IAP-JWT-Assertion`** (a JWT signed by Google) and forwards the request to the **frontend** Cloud Run service.
5. The **frontend** (nginx) serves the SPA and proxies `/api/*` to the **backend** Cloud Run URL, **forwarding** the same `X-Goog-IAP-JWT-Assertion` header.
6. The **backend** validates the JWT (issuer, audience, email) and only accepts requests with an allowed email. No IAP header or wrong identity → 401.

So: **only your Google account** can reach the frontend (IAP), and **only the frontend** can call the backend (internal ingress + IAP JWT check).

## How access is controlled

| Layer            | Control |
|------------------|--------|
| Frontend URL     | IAP: only identities with `roles/iap.httpsResourceAccessor` on the LB backend service. We grant this to a **single user** (`allowed_iap_user_email`). |
| Frontend Cloud Run | No `allUsers`. Only the load balancer identity (`service-*@gcp-sa-runapps.iam.gserviceaccount.com`) has `roles/run.invoker`. Ingress = `INTERNAL_LOAD_BALANCER` so direct `*.run.app` is not accepted. |
| Backend Cloud Run  | No public ingress (`INTERNAL_ONLY`). Only the **frontend** service account has `roles/run.invoker`. Backend also validates the IAP JWT and allowed email. |

## How to add/remove users

- **Single user (current):** The variable `allowed_iap_user_email` is one email. To **change** the user, update the variable and run `terraform apply`. That updates both IAM (`iap.httpsResourceAccessor`) and the backend env `ALLOWED_IAP_EMAILS`.
- **Add more users:** Grant `roles/iap.httpsResourceAccessor` to each new user (e.g. `google_iap_web_backend_service_iam_member` with `member = "user:another@example.com"`). Update `ALLOWED_IAP_EMAILS` in the backend Cloud Run env to include the new emails (e.g. comma-separated in Terraform).
- **Remove a user:** Remove the IAM binding for that user and remove their email from `ALLOWED_IAP_EMAILS`.

## How to rotate service accounts

- **Frontend SA:** Create a new SA in Terraform, grant it `roles/run.invoker` on the **backend** Cloud Run (and remove the old SA from that binding). Update the frontend Cloud Run `template.service_account` to the new SA. Apply. Then you can delete the old SA if unused.
- **Backend SA:** Create a new SA, grant it the same roles (GCS, AlloyDB, Secret Manager). Update the backend Cloud Run `template.service_account` and the backend’s IAM. Apply. Rotate any keys/secrets the old SA had; then delete the old SA if unused.
- **No hardcoded secrets:** All secrets (e.g. DB password) are in Secret Manager; SAs are used for identity only.

## Terraform folder structure

```
terraform/
├── backend.tf      # GCS state backend
├── main.tf         # APIs, VPC, connector, backend CR, frontend CR, LB, IAP, IAM
├── variables.tf    # project_id, region, images, allowed_iap_user_email, frontend_domain
├── outputs.tf      # iap_protected_url, load_balancer_ip, SAs, verification_commands
└── README.md       # This file
```

## Required variables

| Variable | Description |
|----------|-------------|
| `project_id` | GCP project ID |
| `region` | e.g. `europe-west4` |
| `allowed_iap_user_email` | Single Google account allowed (e.g. `you@gmail.com`) |
| `frontend_domain` | Domain for the app (e.g. `album.example.com`). You must create a **DNS A record** pointing to the LB IP after apply. |
| `backend_container_image` | (Optional) Backend image. Empty = use default from Artifact Registry. |
| `frontend_container_image` | (Optional) Frontend image. Empty = use default. |
| `iap_audience` | (Optional) IAP JWT audience so the backend can validate JWTs. **After first apply**, run `terraform output -raw backend_service_audience` and set this variable, then apply again. If empty, the backend does not validate IAP (fine for first apply; set for production). |

## Apply

1. Create a GCS bucket for Terraform state (if not already) and configure `backend.tf`.
2. Create `terraform.tfvars` (or pass `-var`) with `project_id`, `allowed_iap_user_email`, `frontend_domain`.
3. Run:
   ```bash
   terraform init
   terraform plan
   terraform apply
   ```
4. After apply, create a **DNS A record**: `frontend_domain` → `load_balancer_ip` (from `terraform output load_balancer_ip`). Wait for the managed SSL certificate to become ACTIVE (can take several minutes).

## Validation steps

1. **Frontend is inaccessible without Google login**  
   Open `https://<frontend_domain>` in an incognito window. You should get an IAP / Google sign-in page. Without the allowed account, access is denied.

2. **Backend cannot be called directly**  
   The backend Cloud Run URL is internal-only. From your laptop, `curl -sI <backend_run_url>/health` should return **403** (or be unreachable).

3. **Only the allowed Google account works**  
   Sign in with `allowed_iap_user_email` and open `https://<frontend_domain>`. The app should load. Sign in with a different Google account; access should be denied by IAP.

4. **Mobile browser (iPhone/iPad)**  
   Open `https://<frontend_domain>` in Safari (or another browser). Sign in with the allowed account; the app should work without VPN.

## Verification commands (from Terraform output)

Run:

```bash
terraform output -raw verification_commands
```

Then run the printed commands to double-check:

- `curl -sI https://<frontend_domain>` (expect 302/401 or IAP challenge, not 200 without auth).
- Backend URL should not be reachable from the internet.
- Use the allowed account in a browser to confirm access.

## Security rules enforced

- No `allUsers` or `allAuthenticatedUsers` on Cloud Run or IAP.
- No public Cloud Run ingress for the frontend (only LB + internal).
- No public backend; backend is internal-only and validates IAP JWT.
- No hardcoded secrets; use Secret Manager and IAM.
- HTTPS everywhere (managed cert on the LB).
- Explicit IAM only (single IAP user, frontend SA invokes backend, LB identity invokes frontend).
