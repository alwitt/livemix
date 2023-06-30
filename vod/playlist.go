package vod

import (
	"context"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
)

// GetReferenceTime helper function to get the reference time
func GetReferenceTime() (time.Time, error) {
	return time.Parse("2006-Jan-02", "2023-Jan-01")
}

// PlaylistBuilder construct HLS playlist on demand
type PlaylistBuilder interface {
	/*
		GetLiveStreamPlaylist generate a playlist for live stream

			@param ctxt context.Context - execution context
			@param target common.VideoSource - video source to generate the playlist for
			@param timestamp time.Time - current time
			@param addMediaSequence bool - whether to add a media sequence number to the playlist
			@returns the new playlist valid for the given timestamp
	*/
	GetLiveStreamPlaylist(
		ctxt context.Context, target common.VideoSource, timestamp time.Time, addMediaSequence bool,
	) (hls.Playlist, error)
}

// playlistBuilderImpl implements PlaylistBuilder
type playlistBuilderImpl struct {
	goutils.Component
	dbClient db.PersistenceManager
	// segmentDuration number of seconds each segment should have
	segmentDuration time.Duration
	// liveStreamSegCount number of segments to include when building a live stream playlist
	liveStreamSegCount int
	referenceTime      time.Time
}

/*
NewPlaylistBuilder define new playlist builder

	@param dbClient db.PersistenceManager - DB access client
	@param segmentDuration time.Duration - number of seconds each segment should have
	@param liveStreamSegCount int - number of segments to include when building a live stream playlist
	@returns new PlaylistBuilder
*/
func NewPlaylistBuilder(
	dbClient db.PersistenceManager, segmentDuration time.Duration, liveStreamSegCount int,
) (PlaylistBuilder, error) {
	// Define the reference time from which all sequence numbers are built
	referenceTime, err := GetReferenceTime()
	if err != nil {
		return nil, err
	}
	return &playlistBuilderImpl{
		Component: goutils.Component{
			LogTags: log.Fields{"module": "vod", "component": "playlist-builder"},
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		dbClient:           dbClient,
		segmentDuration:    segmentDuration,
		liveStreamSegCount: liveStreamSegCount,
		referenceTime:      referenceTime,
	}, nil
}

func (b *playlistBuilderImpl) GetLiveStreamPlaylist(
	ctxt context.Context, target common.VideoSource, timestamp time.Time, addMediaSequence bool,
) (hls.Playlist, error) {
	logTags := b.GetLogTagsForContext(ctxt)

	// Get the video segments for the source
	segments, err := b.dbClient.GetLatestLiveStreamSegments(ctxt, target.ID, b.liveStreamSegCount)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", target.ID).
			Error("Failed to fetch associated segments")
		return hls.Playlist{}, err
	}

	result := hls.Playlist{
		Name:              target.Name,
		CreatedAt:         timestamp,
		Version:           3,
		TargetSegDuration: b.segmentDuration.Seconds(),
		Segments:          make([]hls.Segment, len(segments)),
	}
	for idx, oneSegment := range segments {
		result.Segments[idx] = oneSegment.Segment
	}

	// Attach a media sequence value
	if addMediaSequence {
		result.AddMediaSequenceVal(b.referenceTime)
	}

	return result, nil
}
