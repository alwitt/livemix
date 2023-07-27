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

/*
TODO FIXME:

Add metrics:
* generate playlist action
	* total playlists - count
	* total segments - count

Labels:
* "type": "live" or "replay"
* "source": video source ID

*/

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

	/*
		GetRecordingStreamPlaylist generate a playlist for a video recording

			@param ctxt context.Context - execution context
			@param recording common.Recording - video recording session
			@returns playlist for the recording session
	*/
	GetRecordingStreamPlaylist(ctxt context.Context, recording common.Recording) (hls.Playlist, error)
}

// playlistBuilderImpl implements PlaylistBuilder
type playlistBuilderImpl struct {
	goutils.Component
	dbConns db.ConnectionManager
	// liveStreamSegCount number of segments to include when building a live stream playlist
	liveStreamSegCount int
	referenceTime      time.Time
}

/*
NewPlaylistBuilder define new playlist builder

	@param dbConns db.ConnectionManager - DB connection manager
	@param liveStreamSegCount int - number of segments to include when building a live stream playlist
	@returns new PlaylistBuilder
*/
func NewPlaylistBuilder(
	dbConns db.ConnectionManager, liveStreamSegCount int,
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
		dbConns:            dbConns,
		liveStreamSegCount: liveStreamSegCount,
		referenceTime:      referenceTime,
	}, nil
}

func (b *playlistBuilderImpl) GetLiveStreamPlaylist(
	ctxt context.Context, target common.VideoSource, timestamp time.Time, addMediaSequence bool,
) (hls.Playlist, error) {
	logTags := b.GetLogTagsForContext(ctxt)

	dbClient := b.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Get the video segments for the source
	segments, err := dbClient.GetLatestLiveStreamSegments(ctxt, target.ID, b.liveStreamSegCount)
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
		TargetSegDuration: float64(target.TargetSegmentLength),
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

func (b *playlistBuilderImpl) GetRecordingStreamPlaylist(
	ctxt context.Context, recording common.Recording,
) (hls.Playlist, error) {
	logTags := b.GetLogTagsForContext(ctxt)

	dbClient := b.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Get the video segments for the recording
	segments, err := dbClient.ListAllSegmentsOfRecording(ctxt, recording.ID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("recording-id", recording.ID).
			Error("Failed to fetch associated segments")
		return hls.Playlist{}, err
	}

	// Get the video source as well
	source, err := dbClient.GetVideoSource(ctxt, recording.SourceID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", recording.SourceID).
			WithField("recording-id", recording.ID).
			Error("Unable to find source associated with recording")
		return hls.Playlist{}, err
	}

	result := hls.Playlist{
		Name:              source.Name,
		CreatedAt:         time.Now().UTC(),
		Version:           3,
		TargetSegDuration: float64(source.TargetSegmentLength),
		Segments:          make([]hls.Segment, len(segments)),
	}
	for idx, oneSegment := range segments {
		result.Segments[idx] = oneSegment.Segment
	}

	defaultMediaSequenceVal := 1
	result.MediaSequenceVal = &defaultMediaSequenceVal

	return result, nil
}
