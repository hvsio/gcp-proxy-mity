# GCP Proxy Service

A Golang microservice for proxying GCP services, starting with Google Cloud Storage support for media files.

## Features

- **Write multiple files**: Upload multiple media files (mp4, heim, jpeg, etc.) to Cloud Storage
- **Write single file (raw)**: Upload raw binary media data directly in request body
- **Read multiple files**: Download multiple files from Cloud Storage
- **Read single file**: Download a single file from Cloud Storage
- Clean architecture with separation of concerns
- Comprehensive unit tests
- Graceful shutdown
- Automatic content-type detection for media files

## Project Structure

```
.
??? cmd/
?   ??? server/          # Application entry point
??? internal/
?   ??? config/          # Configuration management
?   ??? handler/         # HTTP handlers
?   ??? service/         # Business logic layer
?   ??? storage/         # Storage abstraction and GCS implementation
??? pkg/
    ??? storage/
        ??? gcs/         # GCS client wrapper
```

## Prerequisites

- Go 1.21 or higher
- Google Cloud Project with Cloud Storage enabled
- GCS bucket created
- Service account credentials (optional if running on GCP)

## Configuration

The application supports configuration via environment variables or a `.env` file. Environment variables take precedence over `.env` file values.

### Using .env file (Recommended for local development)

1. Copy the example file:
   ```bash
   cp .env_example .env
   ```

2. Edit `.env` with your values:
   ```bash
   GCP_PROJECT_ID=your-project-id
   GCS_BUCKET_NAME=your-bucket-name
   PORT=8080
   GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json
   ```

### Using environment variables

Alternatively, set the following environment variables:

```bash
export GCP_PROJECT_ID="your-project-id"
export GCS_BUCKET_NAME="your-bucket-name"
export PORT="8080"  # Optional, defaults to 8080
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/credentials.json"  # Optional if running on GCP
```

**Note:** The `.env` file is automatically ignored by git (already in `.gitignore`). Use `.env_example` as a template.

## Installation

### Local Development

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build the application
go build -o bin/server ./cmd/server

# Run the application
./bin/server
```

### Docker

```bash
# Build Docker image
docker build -t gcp-proxy-mity:latest .

# Run locally
docker run -p 8080:8080 \
  -e GCP_PROJECT_ID=your-project-id \
  -e GCS_BUCKET_NAME=your-bucket-name \
  gcp-proxy-mity:latest
```

## Deployment to GCP

This application is optimized for deployment to Google Cloud Platform, specifically Cloud Run.

For detailed deployment instructions, see [DEPLOYMENT.md](./DEPLOYMENT.md).

Quick start:
```bash
# Build and deploy to Cloud Run
gcloud builds submit --config cloudbuild.yaml \
  --substitutions _REGION=us-central1,_SERVICE_NAME=gcp-proxy-mity,_GCS_BUCKET_NAME=your-bucket-name
```

## API Endpoints

### Health Check
```
GET /health
```

### Write Files - Multiple Options

#### Option 1: Multipart Form Data (Backward Compatible)
```
POST /api/v1/storage/files
Content-Type: multipart/form-data

Form fields: file1, file2, etc. (or any field names)
```

Example using curl:
```bash
curl -X POST http://localhost:8080/api/v1/storage/files \
  -F "video1=@/path/to/file1.mp4" \
  -F "video2=@/path/to/file2.mp4"
```

#### Option 2: Raw Binary with Path in URL (Recommended for Single Files)
```
PUT /api/v1/storage/files/{filePath}
Content-Type: video/mp4 (or image/jpeg, image/heic, etc.)
Body: Raw binary media data
```

Example using curl:
```bash
# Upload MP4 video
curl -X PUT http://localhost:8080/api/v1/storage/files/videos/my-video.mp4 \
  -H "Content-Type: video/mp4" \
  --data-binary @/path/to/video.mp4

# Upload JPEG image
curl -X PUT http://localhost:8080/api/v1/storage/files/images/photo.jpeg \
  -H "Content-Type: image/jpeg" \
  --data-binary @/path/to/photo.jpeg

# Upload HEIC/HEIM image (content type auto-detected from extension)
curl -X PUT http://localhost:8080/api/v1/storage/files/images/photo.heim \
  --data-binary @/path/to/photo.heim
```

#### Option 3: Raw Binary with Path in Header
```
POST /api/v1/storage/files/raw
Content-Type: video/mp4 (or detected from file extension)
X-File-Path: path/to/file.mp4
Body: Raw binary media data
```

Example using curl:
```bash
curl -X POST http://localhost:8080/api/v1/storage/files/raw \
  -H "Content-Type: video/mp4" \
  -H "X-File-Path: videos/my-video.mp4" \
  --data-binary @/path/to/video.mp4

# Or use query parameter
curl -X POST "http://localhost:8080/api/v1/storage/files/raw?path=videos/my-video.mp4" \
  -H "Content-Type: video/mp4" \
  --data-binary @/path/to/video.mp4
```

**Note:** Content-Type is optional - if not provided, it will be auto-detected from the file extension. Supported formats include:
- **Videos**: mp4, m4v, mov, avi, webm
- **Images**: jpeg, jpg, png, gif, webp, bmp, heic, heim, heif

**Response** (for single file uploads):
```json
{
  "name": "videos/my-video.mp4",
  "content_type": "video/mp4",
  "size": 1234567
}
```

**Response** (for multipart uploads):
```json
{
  "files_written": [
    {
      "name": "video1",
      "content_type": "video/mp4",
      "size": 1234567
    },
    {
      "name": "video2",
      "content_type": "video/mp4",
      "size": 2345678
    }
  ],
  "errors": []
}
```

### Read Multiple Files
```
POST /api/v1/storage/files/read
Content-Type: application/json

Body: {
  "file_paths": ["path/to/file1.mp4", "path/to/file2.mp4"]
}
```

Response:
```json
{
  "files": [
    {
      "metadata": {
        "name": "path/to/file1.mp4",
        "content_type": "video/mp4",
        "size": 1234567
      },
      "content": "<base64 encoded content>"
    }
  ],
  "errors": []
}
```

### Read Single File
```
GET /api/v1/storage/files/{filePath}
```

Example:
```bash
curl http://localhost:8080/api/v1/storage/files/path/to/file.mp4 \
  --output downloaded.mp4
```

## Testing

Run all tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

Run tests for a specific package:
```bash
go test ./internal/storage
go test ./internal/service
go test ./internal/config
```

## Architecture

The application follows clean architecture principles:

1. **Handler Layer** (`internal/handler`): HTTP request/response handling
2. **Service Layer** (`internal/service`): Business logic
3. **Storage Layer** (`internal/storage`): Storage abstraction and GCS implementation
4. **Config** (`internal/config`): Configuration management

This structure allows for:
- Easy testing with mocks
- Swapping storage backends without changing business logic
- Clear separation of concerns

## Error Handling

The service provides detailed error information:

- **WriteFiles**: Returns a list of successfully written files and any errors encountered
- **ReadFiles**: Returns successfully read files and any errors for files that couldn't be read
- All endpoints return appropriate HTTP status codes

## License

MIT