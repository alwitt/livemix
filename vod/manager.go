package vod

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
)

// PlaylistManager VOD controller, and perform prefetching as needed
type PlaylistManager interface {
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

	/*
		GetSegment fetch video segment from cache. If cache miss, get the segment from source.

			@param ctxt context.Context - execution context
			@param target common.VideoSegment - video segment to fetch
			@returns segment data
	*/
	GetSegment(ctxt context.Context, target common.VideoSegment) ([]byte, error)

	/*
		Stop stops the daemon process

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error
}

// playlistManagerImpl implements PlaylistManager
type playlistManagerImpl struct {
	goutils.Component
	plBuilder   PlaylistBuilder
	segmentMgmt SegmentManager

	/* Prefetch related */
	segmentPrefetchCount int
	dbConns              db.ConnectionManager

	/* Support worker related below */
	workers          goutils.TaskProcessor
	wg               sync.WaitGroup
	workerContext    context.Context
	workerCtxtCancel context.CancelFunc
}

/*
NewPlaylistManager define new VOD controller

	@param parentCtxt context.Context - parent context
	@param dbConns db.ConnectionManager - DB connection manager
	@param segPrefetchCount int - max number of recording segment to prefetch in parallel when
	    requests a recording playback
	@param builder PlaylistBuilder - playlist builder
	@param segMgmt SegmentManager - segment manager
	@returns new PlaylistManager
*/
func NewPlaylistManager(
	parentCtxt context.Context,
	dbConns db.ConnectionManager,
	segPrefetchCount int,
	builder PlaylistBuilder,
	segMgmt SegmentManager,
) (PlaylistManager, error) {
	logTags := log.Fields{"module": "vod", "component": "playlist-manager"}

	instance := &playlistManagerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		plBuilder:            builder,
		segmentMgmt:          segMgmt,
		segmentPrefetchCount: segPrefetchCount,
		dbConns:              dbConns,
	}
	instance.workerContext, instance.workerCtxtCancel = context.WithCancel(parentCtxt)

	// Support worker
	var err error
	if instance.workers, err = goutils.GetNewTaskDemuxProcessorInstance(
		instance.workerContext,
		"pl-mgmt-support",
		segPrefetchCount+1,
		segPrefetchCount+1,
		log.Fields{
			"module":        "vod",
			"component":     "playlist-manager",
			"sub-component": "prefetch-worker",
		},
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define support worker")
		return nil, err
	}

	// -----------------------------------------------------------------------------
	// Define support tasks

	if err := instance.workers.AddToTaskExecutionMap(
		reflect.TypeOf(common.VideoSegment{}), instance.prefetchSegment,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to install task definition")
		return nil, err
	}

	// -----------------------------------------------------------------------------
	// Start the worker
	if err := instance.workers.StartEventLoop(&instance.wg); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to start the worker threads")
		return nil, err
	}
	return instance, nil
}

func (m *playlistManagerImpl) Stop(ctxt context.Context) error {
	m.workerCtxtCancel()
	if err := m.workers.StopEventLoop(); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &m.wg, time.Second*5)
}

func (m *playlistManagerImpl) GetLiveStreamPlaylist(
	ctxt context.Context, target common.VideoSource, timestamp time.Time, addMediaSequence bool,
) (hls.Playlist, error) {
	return m.plBuilder.GetLiveStreamPlaylist(ctxt, target, timestamp, addMediaSequence)
}

func (m *playlistManagerImpl) GetSegment(
	ctxt context.Context, target common.VideoSegment,
) ([]byte, error) {
	return m.segmentMgmt.GetSegment(ctxt, target)
}

// ======================================================================================
// Recording replay prefetch

func (m *playlistManagerImpl) GetRecordingStreamPlaylist(
	ctxt context.Context, recording common.Recording,
) (hls.Playlist, error) {
	logTags := m.GetLogTagsForContext(ctxt)

	// Get the playlist first
	newPlaylist, err := m.plBuilder.GetRecordingStreamPlaylist(ctxt, recording)
	if err != nil {
		return hls.Playlist{}, err
	}

	log.
		WithError(err).
		WithFields(logTags).
		WithField("source-id", recording.SourceID).
		WithField("recording-id", recording.ID).
		Info("Submitting segments for prefetching")
	// Perform prefetch
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	for idx, oneSegment := range newPlaylist.Segments {
		if idx == 0 {
			continue
		}
		if idx > m.segmentPrefetchCount {
			// Limit number of prefetched segments.
			// This is also the number of segment readers.
			break
		}
		// Fetch the segment DB entry
		segDBEntry, err := dbClient.GetRecordingSegmentByName(ctxt, oneSegment.Name)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source-id", recording.SourceID).
				WithField("recording-id", recording.ID).
				WithField("segment", oneSegment.Name).
				Error("Unable to read segment DB entry")
			return hls.Playlist{}, err
		}
		if err := m.workers.Submit(ctxt, segDBEntry); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source-id", recording.SourceID).
				WithField("recording-id", recording.ID).
				WithField("segment", oneSegment.Name).
				Error("Failed to submit segment for prefetching")
			return hls.Playlist{}, err
		}
	}
	log.
		WithError(err).
		WithFields(logTags).
		WithField("source-id", recording.SourceID).
		WithField("recording-id", recording.ID).
		Info("Submitted segments for prefetching")

	return newPlaylist, nil
}

func (m *playlistManagerImpl) prefetchSegment(params interface{}) error {
	if request, ok := params.(common.VideoSegment); ok {
		return m.PrefetchSegment(request)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := m.GetLogTagsForContext(m.workerContext)
	log.WithError(err).WithFields(logTags).Error("'prefetchSegment' processing failure")
	return err
}

func (m *playlistManagerImpl) PrefetchSegment(segment common.VideoSegment) error {
	_, err := m.segmentMgmt.GetSegment(m.workerContext, segment)
	return err
}
