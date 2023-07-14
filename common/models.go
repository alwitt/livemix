package common

import (
	"time"

	"github.com/alwitt/livemix/hls"
)

// VideoSource a single HLS video source
type VideoSource struct {
	// ID video source ID
	ID string `json:"id" gorm:"column:id;primaryKey" validate:"required"`
	// Name video source name
	Name string `json:"name" gorm:"column:name;not null;uniqueIndex:video_source_name_index" validate:"required"`
	// TargetSegmentLength expected length of video segments produces by this source
	TargetSegmentLength int `json:"segment_length" gorm:"column:segment_length;not null" validate:"required,gte=1"`
	// Description an optional description of the video source
	Description *string `json:"description,omitempty" gorm:"column:description;default:null"`
	// Streaming whether this source's video is being streamed through the system control node
	Streaming int `json:"streaming" gorm:"column:streaming;default:-1" validate:"oneof=-1 1"`
	// PlaylistURI video source HLS playlist file URI
	PlaylistURI *string `json:"playlist,omitempty" gorm:"column:playlist;default:null" validate:"omitempty,uri"`
	// ReqRespTargetID target ID on the request-response network
	ReqRespTargetID *string `json:"rr_target,omitempty" gorm:"column:rr_target;default:null" validate:"omitempty"`
	// SourceLocalTime local time at the video source
	SourceLocalTime time.Time `json:"local_time" gorm:"column:local_time;default:null"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// VideoSegment a single HLS TS segment
type VideoSegment struct {
	ID string `json:"id" gorm:"column:id;primaryKey;unique" validate:"required"`
	hls.Segment
	// SourceID link to parent video source
	SourceID  string    `json:"source" gorm:"column:source;not null;primaryKey" validate:"required"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// VideoSegmentWithData a single HLS TS segment with its data
type VideoSegmentWithData struct {
	VideoSegment
	Content []byte
}

// Recording represents a video recording session
type Recording struct {
	// ID video recording session ID
	ID string `json:"id" gorm:"column:id;primaryKey" validate:"required"`
	// Alias an optional alias name for the recording session
	Alias *string `json:"alias,omitempty" gorm:"column:alias;default:null"`
	// Description an optional description of the recording session
	Description *string `json:"description,omitempty" gorm:"column:description;default:null"`
	// SourceID ID of the video source this recording session belongs to
	SourceID string `json:"source" gorm:"column:source;not null;index:recording_source_index" validate:"required"`
	// StartTime when the recording session started
	StartTime time.Time `json:"start_ts" validate:"required" gorm:"column:start_ts;not null;index:recording_time_index"`
	// EndTime when the recording session ended
	EndTime time.Time `json:"end_ts" validate:"omitempty" gorm:"column:end_ts;default:null"`
	// Active whether the video recording session is active
	Active    int       `json:"active" gorm:"column:active;default:-1" validate:"oneof=-1 1"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
