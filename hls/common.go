package hls

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Segment represents a HLS TS segment
type Segment struct {
	// Name segment name
	Name string `json:"name" validate:"required" gorm:"column:name;not null;index:video_segment_uniq"`
	// StartTime when segment was first seen
	StartTime time.Time `json:"start" validate:"required" gorm:"column:start_ts;not null"`
	// EndTime end of segment timestamp
	EndTime time.Time `json:"end" validate:"required" gorm:"column:end_ts;not null;index:segment_time_index"`
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
	// TargetSegDuration target segment duration
	TargetSegDuration float64 `json:"duration" validate:"required"`
	// MediaSequenceVal if specified, the "#EXT-X-MEDIA-SEQUENCE:" value
	MediaSequenceVal *int `json:"start_seq_num,omitempty"`
	// Segments list of TS segments associated with this playlist
	Segments []Segment `json:"segments" validate:"required,gt=0,dive"`
}

/*
String toString function for Playlist

	@param continuous bool - whether the playlist should skip the `#EXT-X-ENDLIST` header
	@returns the string representation of a HLS Playlist
*/
func (p Playlist) String(continuous bool) (string, error) {
	builder := strings.Builder{}
	// Write the playlist headers
	for _, oneLine := range []string{
		"#EXTM3U",
		fmt.Sprintf("#EXT-X-VERSION:%d", p.Version),
		fmt.Sprintf("#EXT-X-TARGETDURATION:%f", p.TargetSegDuration),
	} {
		if _, err := builder.WriteString(fmt.Sprintf("%s\n", oneLine)); err != nil {
			return "", err
		}
	}
	// If provided, add "#EXT-X-MEDIA-SEQUENCE:"
	if p.MediaSequenceVal != nil {
		_, err := builder.WriteString(fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d\n", *p.MediaSequenceVal))
		if err != nil {
			return "", err
		}
	}
	// Write the segments
	for _, oneSegment := range p.Segments {
		if _, err := builder.WriteString(
			fmt.Sprintf("#EXTINF:%f,\n%s\n", oneSegment.Length, oneSegment.Name),
		); err != nil {
			return "", err
		}
	}
	// End the playlist
	if !continuous {
		if _, err := builder.WriteString("#EXT-X-ENDLIST\n"); err != nil {
			return "", err
		}
	}
	return builder.String(), nil
}

/*
AddMediaSequenceVal define a media sequence value for the playlist based on a reference time
and the start time of the first segment in the playlist.

	@param reference time.Time - reference time to compare against
*/
func (p *Playlist) AddMediaSequenceVal(reference time.Time) {
	if p.Segments == nil || len(p.Segments) == 0 {
		// No action to name
		p.MediaSequenceVal = nil
		return
	}
	first := p.Segments[0]
	timeDiff := first.StartTime.Sub(reference)
	timeDiffSec := int(timeDiff.Seconds())
	segLenSec := int(p.TargetSegDuration)
	mediaSequenceVal := timeDiffSec / segLenSec
	p.MediaSequenceVal = &mediaSequenceVal
}
