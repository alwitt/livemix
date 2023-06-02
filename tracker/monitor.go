package tracker

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	"github.com/oklog/ulid/v2"
)

// SourceHLSMonitor daemon to monitor a single HLS video source and process its video segments
// as they update
type SourceHLSMonitor interface {
	/*
		Update update status of the HLS source based on information in the current playlist.

		IMPORTANT: The playlist must be from the HLS source being tracked.

			@param ctxt context.Context - execution context
			@param currentPlaylist hls.Playlist - the current playlist for the video source
			@param timestamp time.Time - timestamp of when this update is called
	*/
	Update(ctxt context.Context, currentPlaylist hls.Playlist, timestamp time.Time) error

	/*
		Stop stops the daemon process

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error
}

type segmentReadBatch struct {
	targetSegments []common.VideoSegment
	readContent    map[string][]byte
}

// sourceHLSMonitorImpl implements SourceHLSMonitor
type sourceHLSMonitorImpl struct {
	goutils.Component
	tracker        SourceHLSTracker
	trackingWindow time.Duration
	cache          SourceHLSSegmentCache
	segmentReader  utils.SegmentReader
	ongoingReads   map[string]segmentReadBatch
	forwardSegment SegmentForwardCallback

	/* Support worker related below */
	worker           goutils.TaskProcessor
	wg               sync.WaitGroup
	workerContext    context.Context
	workerCtxtCancel context.CancelFunc
}

// SegmentForwardCallback function signature of callback to send out read video segments
type SegmentForwardCallback func(ctxt context.Context, segment common.VideoSegmentWithData) error

/*
NewSourceHLSMonitor define new single HLS source monitor

Tracking window is the duration in time a video segment is tracked. After observing a new
segment, that segment is remembered for the duration of a tracking window, and forgotten
after that.

	@param parentContext context.Context - context from which to define the worker context
	@param source common.VideoSource - the HLS source to tracker
	@param dbClient db.Manager - DB access client
	@param trackingWindow time.Duration - see note
	@param segmentCache SourceHLSSegmentCache - HLS video segment cache
	@param reader utils.SegmentReader - HLS video segment data reader
	@param forwardSegment SegmentForwardCallback - callback to send out read video segments
	@returns new SourceHLSMonitor
*/
func NewSourceHLSMonitor(
	parentContext context.Context,
	source common.VideoSource,
	dbClient db.PersistenceManager,
	trackingWindow time.Duration,
	segmentCache SourceHLSSegmentCache,
	reader utils.SegmentReader,
	forwardSegment SegmentForwardCallback,
) (SourceHLSMonitor, error) {
	logTags := log.Fields{
		"module":       "tracker",
		"component":    "hls-source-monitor",
		"instance":     source.Name,
		"playlist-uri": source.PlaylistURI,
	}

	// -----------------------------------------------------------------------------
	// Setup components

	// Playlist tracker
	tracker, err := NewSourceHLSTracker(source, dbClient, trackingWindow)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define playlist tracker")
		return nil, err
	}

	// Support worker
	worker, err := goutils.GetNewTaskProcessorInstance(parentContext, "source-monitor", 4, logTags)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define support worker")
		return nil, err
	}

	// Worker context
	workerCtxt, cancel := context.WithCancel(parentContext)

	// Monitor instance
	monitor := &sourceHLSMonitorImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		tracker:          tracker,
		trackingWindow:   trackingWindow,
		cache:            segmentCache,
		segmentReader:    reader,
		ongoingReads:     make(map[string]segmentReadBatch),
		forwardSegment:   forwardSegment,
		worker:           worker,
		wg:               sync.WaitGroup{},
		workerContext:    workerCtxt,
		workerCtxtCancel: cancel,
	}

	// -----------------------------------------------------------------------------
	// Define support tasks

	if err := worker.AddToTaskExecutionMap(
		reflect.TypeOf(monitorUpdateRequest{}), monitor.update,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to install task definition")
		return nil, err
	}

	if err := worker.AddToTaskExecutionMap(
		reflect.TypeOf(monitorSegmentReadNotify{}), monitor.reportSegmentRead,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to install task definition")
		return nil, err
	}

	// -----------------------------------------------------------------------------
	// Start the worker

	if err := worker.StartEventLoop(&monitor.wg); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to start the worker thread")
		return nil, err
	}

	return monitor, nil
}

// ===========================================================================================

type monitorUpdateRequest struct {
	currentPlaylist hls.Playlist
	timestamp       time.Time
}

func (m *sourceHLSMonitorImpl) Update(
	ctxt context.Context, currentPlaylist hls.Playlist, timestamp time.Time,
) error {
	logTags := m.GetLogTagsForContext(m.workerContext)

	// Make the request
	request := monitorUpdateRequest{currentPlaylist: currentPlaylist, timestamp: timestamp}
	if err := m.worker.Submit(ctxt, request); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to submit 'Update' job")
	}

	log.WithFields(logTags).Debug("Submitted 'Update' job request")

	return nil
}

func (m *sourceHLSMonitorImpl) update(params interface{}) error {
	// Convert params into expected data type
	if updateParams, ok := params.(monitorUpdateRequest); ok {
		return m.coreUpdate(updateParams.currentPlaylist, updateParams.timestamp)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := m.GetLogTagsForContext(m.workerContext)
	log.WithError(err).WithFields(logTags).Error("'update' processing failure")
	return err
}

// coreUpdate contains the actual logic for the Update function
func (m *sourceHLSMonitorImpl) coreUpdate(
	currentPlaylist hls.Playlist, timestamp time.Time,
) error {
	logTags := m.GetLogTagsForContext(m.workerContext)

	// Update through the tracker
	newSegments, err := m.tracker.Update(m.workerContext, currentPlaylist, timestamp)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Source tracker update failed")
		return err
	}
	if len(newSegments) == 0 {
		log.WithFields(logTags).Info("The playlist did not define any new segments")
		return nil
	}

	// For the new segments, define a new read batch
	readBatchID := ulid.Make().String()
	newReadBatch := segmentReadBatch{
		targetSegments: newSegments,
		readContent:    make(map[string][]byte),
	}

	// Callback function for the segment reader to pass back the segment
	readSegmentDB := func(ctxt context.Context, segmentID string, content []byte) error {
		return m.ReportSegmentRead(ctxt, readBatchID, segmentID, content)
	}

	// Send out segment read requests
	for _, oneSegment := range newReadBatch.targetSegments {
		parsedURI, err := url.Parse(oneSegment.URI)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("segment-uri", oneSegment.URI).
				Error("Can't parse raw segment URI")
			return err
		}
		if err := m.segmentReader.ReadSegment(
			m.workerContext, oneSegment.ID, parsedURI, readSegmentDB,
		); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("segment-uri", oneSegment.URI).
				Error("Failed to submit segment read request")
			return err
		}
	}
	m.ongoingReads[readBatchID] = newReadBatch
	log.WithFields(logTags).WithField("read-batch-id", readBatchID).Debug("Started new read batch")

	return nil
}

// ===========================================================================================

type monitorSegmentReadNotify struct {
	readID    string
	segmentID string
	content   []byte
}

// ReportSegmentRead to be used as callback by the segment readers after completing a read
func (m *sourceHLSMonitorImpl) ReportSegmentRead(
	ctxt context.Context, readID, segmentID string, content []byte,
) error {
	logTags := m.GetLogTagsForContext(m.workerContext)

	// Update the cache with read segment content
	if err := m.cache.CacheSegment(ctxt, segmentID, content, m.trackingWindow); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment-id", segmentID).
			Error("Faild to cache segment content")
		return err
	}

	// Make the request
	request := monitorSegmentReadNotify{readID: readID, segmentID: segmentID, content: content}
	if err := m.worker.Submit(ctxt, request); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to submit 'ReportSegmentRead' job")
		return err
	}

	log.
		WithFields(logTags).
		WithField("read-batch-id", readID).
		WithField("segment-id", segmentID).
		Debug("Submitted 'ReportSegmentRead' job request")

	return nil
}

func (m *sourceHLSMonitorImpl) reportSegmentRead(params interface{}) error {
	// Convert params into expected data type
	if readReceipt, ok := params.(monitorSegmentReadNotify); ok {
		return m.coreReportSegmentRead(readReceipt.readID, readReceipt.segmentID, readReceipt.content)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := m.GetLogTagsForContext(m.workerContext)
	log.WithError(err).WithFields(logTags).Error("'update' processing failure")
	return err
}

// coreReportSegmentRead contains the actual logic for the ReportSegmentRead function
func (m *sourceHLSMonitorImpl) coreReportSegmentRead(
	readID, segmentID string, content []byte,
) error {
	logTags := m.GetLogTagsForContext(m.workerContext)

	readBatch, ok := m.ongoingReads[readID]
	if !ok {
		err := fmt.Errorf("segment read ID is unknown")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("read-batch-id", readID).
			WithField("segment-id", segmentID).
			Error("Unable to process segment read response")
		return err
	}

	// Update read batch with content
	readBatch.readContent[segmentID] = content

	// Once the batch is complete, report them is correct order
	if len(readBatch.readContent) == len(readBatch.targetSegments) {
		// Sort the segment by start timestamp
		sort.Slice(readBatch.targetSegments, func(i, j int) bool {
			return readBatch.targetSegments[i].StartTime.Before(readBatch.targetSegments[j].StartTime)
		})
		// Forward the segment in ascending order
		for _, segment := range readBatch.targetSegments {
			msg := common.VideoSegmentWithData{
				VideoSegment: segment,
				Content:      readBatch.readContent[segment.ID],
			}
			if err := m.forwardSegment(m.workerContext, msg); err != nil {
				log.
					WithError(err).
					WithFields(logTags).
					WithField("read-batch-id", readID).
					WithField("segment-id", segment.ID).
					Error("Video segment forward failed")
				return err
			}
		}
		// Delete the read batch
		delete(m.ongoingReads, readID)
	}

	return nil
}

// ===========================================================================================

func (m *sourceHLSMonitorImpl) Stop(ctxt context.Context) error {
	m.workerCtxtCancel()
	return m.worker.StopEventLoop()
}
