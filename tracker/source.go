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

// SourceHLSTracker track status of a single HLS video source based on its playlist
type SourceHLSTracker interface {
	/*
		Update update status of the HLS source based on information in the current playlist.

		IMPORTANT: The playlist must be from the HLS source being tracked.

			@param ctxt context.Context - execution context
			@param currentPlaylist hls.Playlist - the current playlist for the video source
			@param timestamp time.Time - timestamp of when this update is called
			@returns the set of new segment described in the playlist
	*/
	Update(
		ctxt context.Context, currentPlaylist hls.Playlist, timestamp time.Time,
	) ([]common.VideoSegment, error)

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
	dbConns        db.ConnectionManager
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
	@param dbConns db.ConnectionManager - DB connection manager
	@param trackingWindow time.Duration - see note
	@returns new SourceHLSTracker
*/
func NewSourceHLSTracker(
	source common.VideoSource, dbConns db.ConnectionManager, trackingWindow time.Duration,
) (SourceHLSTracker, error) {
	return &sourceHLSTrackerImpl{
		Component: goutils.Component{
			LogTags: log.Fields{
				"module":       "tracker",
				"component":    "hls-source-tracker",
				"instance":     source.Name,
				"playlist-uri": source.PlaylistURI,
			},
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		dbConns:        dbConns,
		source:         source,
		validator:      validator.New(),
		trackingWindow: trackingWindow,
	}, nil
}

func (t *sourceHLSTrackerImpl) Update(
	ctxt context.Context, currentPlaylist hls.Playlist, timestamp time.Time,
) ([]common.VideoSegment, error) {
	logTags := t.GetLogTagsForContext(ctxt)

	dbClient := t.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Verify the playlist came from the expected source
	if currentPlaylist.Name != t.source.Name {
		return nil, fmt.Errorf(
			"playlist not from tracked HLS source: %s =/= %s", currentPlaylist.Name, t.source.Name,
		)
	}

	// Get all currently known segments
	allSegments, err := dbClient.ListAllLiveStreamSegments(ctxt, t.source.ID)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to fetch associated segments")
		return nil, err
	}
	allSegmentsByName := map[string]common.VideoSegment{}
	for _, oneSegment := range allSegments {
		allSegmentsByName[oneSegment.Name] = oneSegment
	}

	// Add new segments to DB
	newSegments := []hls.Segment{}
	for _, newSegment := range currentPlaylist.Segments {
		if _, ok := allSegmentsByName[newSegment.Name]; ok {
			continue
		}
		newSegments = append(newSegments, newSegment)
		log.WithFields(logTags).WithField("segment", newSegment.String()).Debug("Observed new segment")
	}
	newSegmentIDs, err := dbClient.BulkRegisterLiveStreamSegments(ctxt, t.source.ID, newSegments)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to record new segments")
		return nil, err
	}

	// Get all currently known segments again
	allSegments, err = dbClient.ListAllLiveStreamSegments(ctxt, t.source.ID)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to fetch associated segments")
		return nil, err
	}
	// Put aside the entries for the new segments to return
	newSegmentsComplete := []common.VideoSegment{}
	for _, oneSegment := range allSegments {
		if _, ok := newSegmentIDs[oneSegment.Name]; ok {
			newSegmentsComplete = append(newSegmentsComplete, oneSegment)
		}
	}

	// Remove segments older than the tracking window
	oldestTime := timestamp.Add(-t.trackingWindow)
	log.WithFields(logTags).Debugf("Dropping segments older than %s", oldestTime.String())
	if err := dbClient.DeleteOldLiveStreamSegments(ctxt, oldestTime); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to drop expired segment")
		return nil, err
	}

	return newSegmentsComplete, nil
}

func (t *sourceHLSTrackerImpl) GetTrackedSegments(ctxt context.Context) ([]common.VideoSegment, error) {
	dbClient := t.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.ListAllLiveStreamSegments(ctxt, t.source.ID)
}
