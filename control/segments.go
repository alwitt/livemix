package control

import (
	"context"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
)

/*
TODO FIXME:

Add metrics:
* segment register action:
	* total segments - count
	* total bytes - count

Labels:
* "source": video source ID

*/

// LiveStreamSegmentManager central video segment manager which operates within the
// system control node
type LiveStreamSegmentManager interface {
	/*
		Ready check whether the manager is ready

			@param ctxt context.Context - execution context
	*/
	Ready(ctxt context.Context) error

	// =====================================================================================
	// Live Stream Video segments

	/*
		RegisterLiveStreamSegment record a new segment with a source

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param segment hls.Segment - video segment parameters
	*/
	RegisterLiveStreamSegment(
		ctxt context.Context, sourceID string, segment hls.Segment, content []byte,
	) error

	// =====================================================================================
	// Utilities

	/*
		Stop stop all background operations

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error
}

// centrlSegmentManager implements LiveStreamSegmentManager
type liveStreamSegmentManagerImpl struct {
	goutils.Component
	dbConns          db.ConnectionManager
	cache            utils.VideoSegmentCache
	trackingWindow   time.Duration
	supportTimer     goutils.IntervalTimer
	workerCtxt       context.Context
	workerCtxtCancel context.CancelFunc
	wg               sync.WaitGroup
}

/*
NewLiveStreamSegmentManager define a new live stream segment manager

	@param parentCtxt context.Context - parent context
	@param dbConns db.ConnectionManager - DB connection manager
	@param cache utils.VideoSegmentCache - video segment cache
	@param trackingWindow time.Duration - tracking window is the duration in time a video
	    segment is tracked. Recorded segments are forgotten after this tracking window.
	@returns new manager
*/
func NewLiveStreamSegmentManager(
	parentCtxt context.Context,
	dbConns db.ConnectionManager,
	cache utils.VideoSegmentCache,
	trackingWindow time.Duration,
) (LiveStreamSegmentManager, error) {
	logTags := log.Fields{"module": "control", "component": "segment-manager"}

	instance := &liveStreamSegmentManagerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		dbConns:        dbConns,
		cache:          cache,
		trackingWindow: trackingWindow,
		wg:             sync.WaitGroup{},
	}
	instance.workerCtxt, instance.workerCtxtCancel = context.WithCancel(parentCtxt)

	// Define support timer
	timer, err := goutils.GetIntervalTimerInstance(instance.workerCtxt, &instance.wg, logTags)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define support timer")
		return nil, err
	}
	instance.supportTimer = timer

	// Start time to periodically purge old segments
	if err := timer.Start(trackingWindow, instance.purgeOldSegments, false); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to start support timer")
		return nil, err
	}

	return instance, nil
}

func (m *liveStreamSegmentManagerImpl) Ready(ctxt context.Context) error {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.Ready(ctxt)
}

func (m *liveStreamSegmentManagerImpl) RegisterLiveStreamSegment(
	ctxt context.Context, sourceID string, segment hls.Segment, content []byte,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Persist segment record
	segmentID, err := dbClient.RegisterLiveStreamSegment(ctxt, sourceID, segment)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("segment-name", segment.Name).
			Error("Failed to record segment")
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", sourceID).
		WithField("segment-name", segment.Name).
		WithField("segment-id", segmentID).
		Debug("Recorded new segment")

	// Cache segment record
	if err := m.cache.CacheSegment(ctxt, segmentID, content, m.trackingWindow); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("segment-name", segment.Name).
			WithField("segment-id", segmentID).
			Error("Unable to cache segment")
		return err
	}

	return nil
}

func (m *liveStreamSegmentManagerImpl) purgeOldSegments() error {
	timeLimit := time.Now().UTC().Add(-m.trackingWindow)
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.PurgeOldLiveStreamSegments(m.workerCtxt, timeLimit)
}

func (m *liveStreamSegmentManagerImpl) Stop(ctxt context.Context) error {
	m.workerCtxtCancel()
	if err := m.supportTimer.Stop(); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &m.wg, time.Second*5)
}
