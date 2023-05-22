package db

import (
	"github.com/alwitt/livemix/common"
)

// videoSource a single HLS video source
type videoSource struct {
	common.VideoSource
	// Segments associated video segments
	Segments []videoSegment `gorm:"foreignKey:SourceID"`
}

// TableName hard code table name
func (videoSource) TableName() string {
	return "video_sources"
}

// videoSegment a single HLS TS segment
type videoSegment struct {
	common.VideoSegment
	Source videoSource `gorm:"constraint:OnDelete:CASCADE;foreignKey:SourceID"`
}

// TableName hard code table name
func (videoSegment) TableName() string {
	return "video_segments"
}
