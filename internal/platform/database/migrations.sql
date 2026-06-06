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

-- First-party photo library assets. These are the v1 product records used by uni-album.
CREATE TABLE IF NOT EXISTS photo_assets (
    id VARCHAR(255) PRIMARY KEY,
    filename VARCHAR(512) NOT NULL,
    media_type VARCHAR(20) NOT NULL CHECK (media_type IN ('photo', 'video')),
    mime_type VARCHAR(100) NOT NULL,
    size BIGINT NOT NULL,
    original_object_key VARCHAR(1000) NOT NULL UNIQUE,
    preview_object_key VARCHAR(1000),
    uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    metadata JSONB NOT NULL DEFAULT '{}',
    favorite BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_photo_assets_uploaded_at ON photo_assets(uploaded_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_photo_assets_media_type ON photo_assets(media_type);
CREATE INDEX IF NOT EXISTS idx_photo_assets_favorite ON photo_assets(favorite);
CREATE INDEX IF NOT EXISTS idx_photo_assets_metadata ON photo_assets USING GIN(metadata);

CREATE TABLE IF NOT EXISTS photo_folders (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    parent_id VARCHAR(255) REFERENCES photo_folders(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_photo_folders_parent_id ON photo_folders(parent_id);

CREATE TABLE IF NOT EXISTS photo_albums (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    cover_emoji VARCHAR(32) NOT NULL DEFAULT '📷',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS photo_album_assets (
    album_id VARCHAR(255) NOT NULL REFERENCES photo_albums(id) ON DELETE CASCADE,
    asset_id VARCHAR(255) NOT NULL REFERENCES photo_assets(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (album_id, asset_id)
);

CREATE INDEX IF NOT EXISTS idx_photo_album_assets_asset_id ON photo_album_assets(asset_id);

CREATE TABLE IF NOT EXISTS photo_jobs (
    id VARCHAR(255) PRIMARY KEY,
    type VARCHAR(100) NOT NULL,
    asset_id VARCHAR(255) REFERENCES photo_assets(id) ON DELETE CASCADE,
    state VARCHAR(32) NOT NULL CHECK (state IN ('queued', 'running', 'succeeded', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_photo_jobs_state ON photo_jobs(state);
CREATE INDEX IF NOT EXISTS idx_photo_jobs_asset_id ON photo_jobs(asset_id);
CREATE INDEX IF NOT EXISTS idx_photo_jobs_created_at ON photo_jobs(created_at DESC);
