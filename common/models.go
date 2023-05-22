package common

import (
	"time"

	"github.com/alwitt/livemix/hls"
)

// VideoSource a single HLS video source
type VideoSource struct {
	ID          string  `json:"id" gorm:"column:id;primaryKey" validate:"required"`
	Name        string  `json:"name" gorm:"column:name;not null;uniqueIndex:video_source_name_index" validate:"required"`
	Description *string `json:"description,omitempty" gorm:"column:description;default:null"`
	// PlaylistURI video source HLS playlist file URI
	PlaylistURI string    `json:"playlist" gorm:"column:playlist;not null" validate:"required,url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VideoSegment a single HLS TS segment
type VideoSegment struct {
	ID string `json:"id" gorm:"column:id;primaryKey" validate:"required"`
	hls.Segment
	// SourceID link to parent video source
	SourceID  string    `json:"source" gorm:"column:source;not null;index:video_segment_uniq" validate:"required"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
