-- Media metadata table to store file metadata and EXIF data
CREATE TABLE IF NOT EXISTS media_metadata (
    id VARCHAR(255) PRIMARY KEY,
    file_path VARCHAR(1000) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    exif_data JSONB DEFAULT '{}',
    tags JSONB DEFAULT '[]',
    user_id VARCHAR(255) NOT NULL,
    is_deleted BOOLEAN DEFAULT FALSE
);

-- Index for efficient queries
CREATE INDEX IF NOT EXISTS idx_media_metadata_user_id ON media_metadata(user_id);
CREATE INDEX IF NOT EXISTS idx_media_metadata_file_path ON media_metadata(file_path);
CREATE INDEX IF NOT EXISTS idx_media_metadata_created_at ON media_metadata(created_at);
CREATE INDEX IF NOT EXISTS idx_media_metadata_is_deleted ON media_metadata(is_deleted);
CREATE INDEX IF NOT EXISTS idx_media_metadata_tags ON media_metadata USING GIN(tags);

-- Unique constraint to prevent duplicate file paths per user
CREATE UNIQUE INDEX IF NOT EXISTS idx_media_metadata_unique_file_user 
ON media_metadata(file_path, user_id) WHERE is_deleted = FALSE;

-- Signed URLs table for temporary access
CREATE TABLE IF NOT EXISTS signed_urls (
    id VARCHAR(255) PRIMARY KEY,
    file_path VARCHAR(1000) NOT NULL,
    url TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    user_id VARCHAR(255) NOT NULL,
    purpose VARCHAR(50) NOT NULL CHECK (purpose IN ('read', 'write', 'delete'))
);

-- Index for efficient cleanup and queries
CREATE INDEX IF NOT EXISTS idx_signed_urls_expires_at ON signed_urls(expires_at);
CREATE INDEX IF NOT EXISTS idx_signed_urls_user_id ON signed_urls(user_id);
CREATE INDEX IF NOT EXISTS idx_signed_urls_file_path ON signed_urls(file_path);
