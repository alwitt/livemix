package hls

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"time"
)

// Segment represents a HLS TS segment
type Segment struct {
	// Name segment name
	Name string `json:"name" validate:"required" gorm:"column:name;not null;index:video_segment_uniq"`
	// StartTime when segment was first seen
	StartTime time.Time `json:"start" validate:"required" gorm:"column:start;not null"`
	// EndTime end of segment timestamp
	EndTime time.Time `json:"end" validate:"required" gorm:"column:end;not null;index:segment_time_index"`
	// Length segment length in time
	Length float64 `json:"length" validate:"required" gorm:"column:length;not null"`
	// URI video segment storage URI
	URI string `json:"uri" validate:"required,uri" gorm:"column:uri;not null"`
}

// String toString function
func (s Segment) String() string {
	t, _ := json.Marshal(&s)
	return string(t)
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
func (p Playlist) GetDIRPath() string {
	dirPath, _ := filepath.Split(p.URI.EscapedPath())
	return dirPath
}

/*
BuildSegmentURI helper function to get the segment URI

	@param segmentName string - name of the segment object
	@return segment URI
*/
func (p Playlist) BuildSegmentURI(segmentName string) string {
	dirPath, _ := filepath.Split(p.URI.EscapedPath())
	objectPath := filepath.Join(dirPath, segmentName)
	// Build new URI
	newURI := *p.URI
	newURI.Path = objectPath
	return newURI.String()
}
