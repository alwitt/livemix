package db

import (
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/hls"
)

// videoSource a single HLS video source
type videoSource struct {
	common.VideoSource
	// Segments associated video segments
	Segments []VideoSegment `gorm:"foreignKey:SourceID"`
}

// TableName hard code table name
func (videoSource) TableName() string {
	return "video_sources"
}

// VideoSegment a single HLS TS segment
type VideoSegment struct {
	ID string `gorm:"column:id;primaryKey"`
	hls.Segment
	// StorageURI video segment storage URI
	StorageURI string `gorm:"column:storage;not null"`
	// SourceID link to parent video source
	SourceID  string      `gorm:"column:source;not null;index:video_segment_source_id"`
	Source    videoSource `gorm:"constraint:OnDelete:CASCADE;foreignKey:SourceID"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName hard code table name
func (VideoSegment) TableName() string {
	return "video_segments"
}
