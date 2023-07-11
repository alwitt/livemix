package common

import (
	"time"

	"github.com/alwitt/livemix/hls"
)

// VideoSource a single HLS video source
type VideoSource struct {
	ID                  string  `json:"id" gorm:"column:id;primaryKey" validate:"required"`
	Name                string  `json:"name" gorm:"column:name;not null;uniqueIndex:video_source_name_index" validate:"required"`
	TargetSegmentLength int     `json:"segment_length" gorm:"column:segment_length;not null" validate:"required,gte=1"`
	Description         *string `json:"description,omitempty" gorm:"column:description;default:null"`
	Streaming           int     `json:"streaming" gorm:"column:streaming;default:-1" validate:"oneof=-1 1"`
	// PlaylistURI video source HLS playlist file URI
	PlaylistURI     *string   `json:"playlist,omitempty" gorm:"column:playlist;default:null" validate:"omitempty,uri"`
	ReqRespTargetID *string   `json:"rr_target,omitempty" gorm:"column:rr_target;default:null" validate:"omitempty"`
	SourceLocalTime time.Time `json:"local_time" gorm:"column:local_time;default:null"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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

// VideoSegmentWithData a single HLS TS segment with its data
type VideoSegmentWithData struct {
	VideoSegment
	Content []byte
}
