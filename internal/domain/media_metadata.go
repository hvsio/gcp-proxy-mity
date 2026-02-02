package domain

import "time"

// ==============================
// Asset Metadata (Write Model)
// ==============================

type AssetMetadata struct {
	// Identity
	AssetID   string    `json:"assetId"`
	TakenAt   time.Time `json:"takenAt"`   // DateTimeOriginal (UTC)
	CreatedAt time.Time `json:"createdAt"` // Upload time

	// Classification
	MediaType MediaType `json:"mediaType"` // photo | video | live
	Format    string    `json:"format"`    // heic | jpeg | png | mov

	// Image / video properties
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Orientation int    `json:"orientation"`
	ColorSpace  string `json:"colorSpace"` // DisplayP3 | sRGB

	// Capture device
	Camera CameraMetadata `json:"camera"`

	// Exposure & capture details
	Exposure ExposureMetadata `json:"exposure"`

	// Optional location data
	Location *LocationMetadata `json:"location,omitempty"`

	// Computational photography flags
	Computational ComputationalMetadata `json:"computational"`

	// Live Photo linkage
	LivePhoto *LivePhotoMetadata `json:"livePhoto,omitempty"`

	// Opaque vendor-specific metadata (Apple MakerNotes, HEIC aux data, etc.)
	RawMetadata map[string]any `json:"rawMetadata,omitempty"`
}

// ==============================
// Enums & Constants
// ==============================

type MediaType string

const (
	MediaTypePhoto MediaType = "photo"
	MediaTypeVideo MediaType = "video"
	MediaTypeLive  MediaType = "live"
)

// ==============================
// Camera Metadata
// ==============================

type CameraMetadata struct {
	Make          string  `json:"make"`  // Apple
	Model         string  `json:"model"` // iPhone 13 Pro Max
	Lens          string  `json:"lens"`  // wide | ultra-wide | telephoto
	FocalLengthMM float64 `json:"focalLengthMm"`
}

// ==============================
// Exposure Metadata
// ==============================

type ExposureMetadata struct {
	ISO          int     `json:"iso"`
	ShutterSec   float64 `json:"shutterSec"` // 0.001 = 1/1000s
	Aperture     float64 `json:"aperture"`   // f-number
	ExposureBias float64 `json:"exposureBias"`
	FlashFired   bool    `json:"flashFired"`
	WhiteBalance string  `json:"whiteBalance"` // auto | manual
}

// ==============================
// Location Metadata
// ==============================

type LocationMetadata struct {
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	AltitudeM  float64   `json:"altitudeM,omitempty"`
	AccuracyM  float64   `json:"accuracyM,omitempty"`
	CapturedAt time.Time `json:"capturedAt"`
}

// ==============================
// Computational Photography
// ==============================

type ComputationalMetadata struct {
	HDR       bool `json:"hdr"`
	Portrait  bool `json:"portrait"`
	NightMode bool `json:"nightMode"`
	DepthMap  bool `json:"depthMapPresent"`
}

// ==============================
// Live Photo Metadata
// ==============================

type LivePhotoMetadata struct {
	ContentID     string        `json:"contentId"` // Links photo + video
	VideoDuration time.Duration `json:"videoDuration"`
}
