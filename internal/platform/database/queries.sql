-- name: CreatePhotoAsset :exec
INSERT INTO photo_assets (
    id,
    filename,
    media_type,
    mime_type,
    size,
    original_object_key,
    preview_object_key,
    uploaded_at,
    metadata,
    favorite
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: GetPhotoAsset :one
SELECT
    id,
    filename,
    media_type,
    mime_type,
    size,
    original_object_key,
    preview_object_key,
    uploaded_at,
    metadata,
    favorite
FROM photo_assets
WHERE id = $1;

-- name: ListPhotoAssets :many
SELECT
    a.id,
    a.filename,
    a.media_type,
    a.mime_type,
    a.size,
    a.original_object_key,
    a.preview_object_key,
    a.uploaded_at,
    a.metadata,
    a.favorite
FROM photo_assets a
WHERE (sqlc.arg(album_id)::text = '' OR EXISTS (
    SELECT 1
    FROM photo_album_assets aa
    WHERE aa.album_id = sqlc.arg(album_id)::text
      AND aa.asset_id = a.id
))
ORDER BY a.uploaded_at DESC, a.id DESC
LIMIT sqlc.arg(asset_limit)::int
OFFSET sqlc.arg(asset_offset)::int;

-- name: SetPhotoAssetFavorite :one
UPDATE photo_assets
SET favorite = $2
WHERE id = $1
RETURNING
    id,
    filename,
    media_type,
    mime_type,
    size,
    original_object_key,
    preview_object_key,
    uploaded_at,
    metadata,
    favorite;

-- name: CreatePhotoAlbum :exec
INSERT INTO photo_albums (
    id,
    name,
    cover_emoji,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5);

-- name: ListPhotoAlbums :many
SELECT
    a.id,
    a.name,
    a.cover_emoji,
    a.created_at,
    a.updated_at,
    COUNT(aa.asset_id)::int AS asset_count
FROM photo_albums a
LEFT JOIN photo_album_assets aa ON aa.album_id = a.id
GROUP BY a.id
ORDER BY a.created_at ASC;

-- name: UpdatePhotoAlbum :execrows
UPDATE photo_albums
SET
    name = $2,
    cover_emoji = $3,
    updated_at = $4
WHERE id = $1;

-- name: DeletePhotoAlbum :execrows
DELETE FROM photo_albums
WHERE id = $1;

-- name: AddPhotoAssetToAlbum :exec
INSERT INTO photo_album_assets (
    album_id,
    asset_id
)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemovePhotoAssetFromAlbum :exec
DELETE FROM photo_album_assets
WHERE album_id = $1
  AND asset_id = $2;

-- name: CreatePhotoJob :exec
INSERT INTO photo_jobs (
    id,
    type,
    asset_id,
    state,
    attempts,
    error,
    created_at,
    updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListPhotoJobs :many
SELECT
    id,
    type,
    asset_id,
    state,
    attempts,
    error,
    created_at,
    updated_at
FROM photo_jobs
ORDER BY created_at DESC
LIMIT $1;
