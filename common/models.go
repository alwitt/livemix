package common

import "time"

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
