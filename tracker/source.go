package tracker

import (
	"context"
	"fmt"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
)

// SourceHLSTracker monitor status of a single HLS video source
type SourceHLSTracker interface {
	/*
		Update update status of the HLS source based on information in the current playlist.

		IMPORTANT: The playlist must be from the HLS source being tracked.

			@param ctxt context.Context - execution context
			@param currentPlaylist hls.Playlist - the current playlist for the video source
			@param timestamp time.Time - timestamp of when this update is called
	*/
	Update(ctxt context.Context, currentPlaylist hls.Playlist, timestamp time.Time) error

	/*
		GetTrackedSegments fetch the list of tracked segments

			@param ctxt context.Context - execution context
			@returns tracked video segments
	*/
	GetTrackedSegments(ctxt context.Context) ([]common.VideoSegment, error)
}

// sourceHLSTrackerImpl implements SourceHLSTracker
type sourceHLSTrackerImpl struct {
	goutils.Component
	dbClient       db.PersistenceManager
	source         common.VideoSource
	validator      *validator.Validate
	trackingWindow time.Duration
}

/*
NewSourceHLSTracker define new single HLS source tracker

Tracking window is the duration in time a video segment is tracked. After observing a new
segment, that segment is remembered for the duration of a tracking window, and forgotten
after that.

	@param source common.VideoSource - the HLS source to tracker
	@param dbClient db.Manager - DB access client
	@param trackingWindow time.Duration - see note
*/
func NewSourceHLSTracker(
	source common.VideoSource, dbClient db.PersistenceManager, trackingWindow time.Duration,
) (SourceHLSTracker, error) {
	logTags := log.Fields{
		"module":       "tracker",
		"component":    "hls-source-tracker",
		"instance":     source.Name,
		"playlist-uri": source.PlaylistURI,
	}

	return &sourceHLSTrackerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		dbClient:       dbClient,
		source:         source,
		validator:      validator.New(),
		trackingWindow: trackingWindow,
	}, nil
}

func (t *sourceHLSTrackerImpl) Update(
	ctxt context.Context, currentPlaylist hls.Playlist, timestamp time.Time,
) error {
	logTags := t.GetLogTagsForContext(ctxt)

	// Verify the playlist came from the expected source
	if currentPlaylist.Name != t.source.Name || currentPlaylist.URI.String() != t.source.PlaylistURI {
		return fmt.Errorf(
			"playlist not from tracked HLS source: %s =/= %s",
			currentPlaylist.URI.String(),
			t.source.PlaylistURI,
		)
	}

	// Get all currently known segments
	allSegments, err := t.dbClient.ListAllSegmentsBeforeTime(ctxt, t.source.ID, timestamp)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to fetch associated segments")
		return err
	}
	segmentByName := map[string]common.VideoSegment{}
	for _, oneSegment := range allSegments {
		segmentByName[oneSegment.Name] = oneSegment
	}

	// Add new segments to DB
	newSegments := []hls.Segment{}
	for _, newSegment := range currentPlaylist.Segments {
		if _, ok := segmentByName[newSegment.Name]; ok {
			continue
		}
		newSegments = append(newSegments, newSegment)
		log.WithFields(logTags).WithField("segment", newSegment.String()).Debug("Observed new segment")
	}
	if _, err = t.dbClient.BulkRegisterSegments(ctxt, t.source.ID, newSegments); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to record new segments")
		return err
	}

	// Get all currently known segments again
	allSegments, err = t.dbClient.ListAllSegmentsBeforeTime(ctxt, t.source.ID, timestamp)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to fetch associated segments")
		return err
	}

	// Remove segments older than the tracking window
	oldestTime := timestamp.Add(-t.trackingWindow)
	log.WithFields(logTags).Debugf("Dropping segments older than %s", oldestTime.String())
	deleteSegments := []string{}
	for _, oneSegment := range allSegments {
		if oldestTime.After(oneSegment.EndTime) {
			deleteSegments = append(deleteSegments, oneSegment.ID)
			log.
				WithFields(logTags).
				WithField("segment", oneSegment.String()).
				Debug("Dropping expired segment")
		}
	}
	if len(deleteSegments) > 0 {
		if err := t.dbClient.BulkDeleteSegment(ctxt, deleteSegments); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to drop expired segment")
			return err
		}
	}

	return nil
}

func (t *sourceHLSTrackerImpl) GetTrackedSegments(ctxt context.Context) ([]common.VideoSegment, error) {
	return t.dbClient.ListAllSegments(ctxt, t.source.ID)
}
