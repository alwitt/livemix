package db

import (
	"github.com/alwitt/livemix/common"
)

// videoSource a single HLS video source
type videoSource struct {
	common.VideoSource
	// Segments associated video segments
	Segments []liveStreamVideoSegment `gorm:"foreignKey:SourceID" validate:"-"`
}

// TableName hard code table name
func (videoSource) TableName() string {
	return "video_sources"
}

// liveStreamVideoSegment MPEG-TS video segment belonging to a live stream which the system
// will clear out periodically.
type liveStreamVideoSegment struct {
	common.VideoSegment
	Source videoSource `gorm:"constraint:OnDelete:CASCADE;foreignKey:SourceID" validate:"-"`
}

// TableName hard code table name
func (liveStreamVideoSegment) TableName() string {
	return "live_video_segments"
}

// recordingSession a single video recording session
type recordingSession struct {
	common.Recording
	Segments []*recordingVideoSegment `gorm:"many2many:recording_session_segments;"`
}

// TableName hard code table name
func (recordingSession) TableName() string {
	return "recording_sessions"
}

// recordingVideoSegment MPEG-TS video segment belonging to a video recording session
type recordingVideoSegment struct {
	common.VideoSegment
	Recordings []*recordingSession `gorm:"many2many:recording_session_segments;"`
}

// TableName hard code table name
func (recordingVideoSegment) TableName() string {
	return "recorded_segments"
}
