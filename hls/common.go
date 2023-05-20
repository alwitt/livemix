package hls

import (
	"net/url"
	"path/filepath"
	"time"
)

// Segment represents a HLS TS segment
type Segment struct {
	// Name segment name
	Name string `json:"name" validate:"required"`
	// StartTime when segment was first seen
	StartTime time.Time `json:"start" validate:"required"`
	// EndTime end of segment timestamp
	EndTime time.Time `json:"end" validate:"required"`
	// Length segment length in time
	Length float64 `json:"length" validate:"required"`
}

/*
GetDuration helper function to convert `Length` to a `time.Duration` field.

	@returns segment duration
*/
func (s Segment) GetDuration() time.Duration {
	return time.Duration(float64(time.Second) * s.Length)
}

// Playlist represents a HLS playlist
type Playlist struct {
	// Name video playlist name
	Name string `json:"name" validate:"required"`
	// CreatedAt when the playlist was created
	CreatedAt time.Time `json:"created_at" validate:"required"`
	// Version EXT-X-VERSION value
	Version int `json:"version"`
	// URI video playlist file URI
	URI *url.URL `json:"path" validate:"required,uri"`
	// TargetSegDuration target segment duration
	TargetSegDuration float64 `json:"duration" validate:"required"`
	// Segments list of TS segments associated with this playlist
	Segments []Segment `json:"segments" validate:"required,gt=0,dive"`
}

/*
GetDIRPath helper function to get the DIR where the playlist is

	@returns DIR path
*/
func (p Playlist) GetDIRPath() (string, error) {
	dirPath, _ := filepath.Split(p.URI.EscapedPath())
	return dirPath, nil
}
