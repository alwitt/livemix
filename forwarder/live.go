package forwarder

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/apex/log"
)

// LiveStreamSegmentForwarder forward video segments in support of a live streaming
// a video source
type LiveStreamSegmentForwarder interface {
	/*
		Stop stop any background support tasks

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error

	/*
		ForwardSegment process a new segment for forwarding

			@param ctxt context.Context - execution context
			@param segment common.VideoSegmentWithData - segment to process
			@param blocking bool - block call until the forwarding complete
	*/
	ForwardSegment(ctxt context.Context, segment common.VideoSegmentWithData, blocking bool) error
}

// httpLiveStreamSegmentForwarder HTTP version of LiveStreamSegmentForwarder
type httpLiveStreamSegmentForwarder struct {
	goutils.Component
	dbConns          db.ConnectionManager
	client           SegmentSender
	workers          goutils.TaskProcessor
	wg               sync.WaitGroup
	workerCtxt       context.Context
	workerCtxtCancel context.CancelFunc
}

/*
NewHTTPLiveStreamSegmentForwarder define new HTTP version of LiveStreamSegmentForwarder

	@param parentCtxt context.Context - forwarder's parent execution context
	@param dbConns db.ConnectionManager - DB connection manager
	@param sender SegmentSender - client for forwarding video segments to system control node
	@param maxInFlightSegments int - max number of segment being forwarded at any one time
	@param tpMetrics goutils.TaskProcessorMetricHelper - task processor metrics helper
	@returns new LiveStreamSegmentForwarder
*/
func NewHTTPLiveStreamSegmentForwarder(
	parentCtxt context.Context,
	dbConns db.ConnectionManager,
	sender SegmentSender,
	maxInFlightSegments int,
	tpMetrics goutils.TaskProcessorMetricHelper,
) (LiveStreamSegmentForwarder, error) {
	logTags := log.Fields{
		"module":    "forwarder",
		"component": "http-segment-forwarder",
	}

	instance := &httpLiveStreamSegmentForwarder{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		dbConns: dbConns,
		client:  sender,
		wg:      sync.WaitGroup{},
	}

	// Worker context
	instance.workerCtxt, instance.workerCtxtCancel = context.WithCancel(parentCtxt)

	// Support worker
	workerLogsTags := log.Fields{
		"module":     "forwarder",
		"component":  "http-segment-forwarder",
		"sub-module": "support-worker",
	}
	workers, err := goutils.GetNewTaskDemuxProcessorInstance(
		instance.workerCtxt,
		"segment-forwarder-core",
		maxInFlightSegments+1,
		maxInFlightSegments+1,
		workerLogsTags,
		tpMetrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define support worker")
		return nil, err
	}
	instance.workers = workers

	// -----------------------------------------------------------------------------
	// Define support tasks

	if err := workers.AddToTaskExecutionMap(
		reflect.TypeOf(liveSegFwdForwardReq{}), instance.forwardSegment,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to install task definition")
		return nil, err
	}

	// -----------------------------------------------------------------------------
	// Start the worker

	if err := workers.StartEventLoop(&instance.wg); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to start the worker threads")
		return nil, err
	}

	return instance, nil
}

func (f *httpLiveStreamSegmentForwarder) Stop(ctxt context.Context) error {
	f.workerCtxtCancel()
	if err := f.workers.StopEventLoop(); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &f.wg, time.Second*10)
}

// ======================================================================================
// Segment Processing

type liveSegFwdForwardReq struct {
	Segment  common.VideoSegmentWithData
	Feedback chan error
}

func (f *httpLiveStreamSegmentForwarder) ForwardSegment(
	ctxt context.Context, segment common.VideoSegmentWithData, blocking bool,
) error {
	logTags := f.GetLogTagsForContext(ctxt)

	feedback := make(chan error, 1)

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment-name", segment.Name).
		Debug("Submitting new segment for processing")

	err := f.workers.Submit(ctxt, liveSegFwdForwardReq{Segment: segment, Feedback: feedback})
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segment.SourceID).
			WithField("segment-name", segment.Name).
			Error("Failed to submit new segment for processing")
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", segment.SourceID).
		WithField("segment-name", segment.Name).
		Debug("New segment submitted for processing")

	if blocking {
		select {
		case <-ctxt.Done():
			return fmt.Errorf("context timed out before 'ForwardSegment' completed")
		case fwdErr, ok := <-feedback:
			if !ok {
				return fmt.Errorf("failed to receive segment forward status feedback")
			}
			return fwdErr
		}
	}

	return nil
}

func (f *httpLiveStreamSegmentForwarder) forwardSegment(params interface{}) error {
	// Convert params into expected data type
	if segment, ok := params.(liveSegFwdForwardReq); ok {
		return f.handleForwardSegment(segment)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := f.GetLogTagsForContext(f.workerCtxt)
	log.WithError(err).WithFields(logTags).Error("'ForwardSegment' processing failure")
	return err
}

func (f *httpLiveStreamSegmentForwarder) handleForwardSegment(
	segmentWrapper liveSegFwdForwardReq,
) error {
	logTags := f.GetLogTagsForContext(f.workerCtxt)

	dbClient := f.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Verify forwarding is enabled first
	source, err := dbClient.GetVideoSource(f.workerCtxt, segmentWrapper.Segment.SourceID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segmentWrapper.Segment.SourceID).
			WithField("segment-name", segmentWrapper.Segment.Name).
			Error("Unable to find the video source entry in persistence")
		segmentWrapper.Feedback <- err
		return err
	}
	if source.Streaming != 1 {
		log.
			WithFields(logTags).
			WithField("source-id", segmentWrapper.Segment.SourceID).
			WithField("segment-name", segmentWrapper.Segment.Name).
			Debug("Video source is not instructed to forward video segments")
		segmentWrapper.Feedback <- nil
		return nil
	}

	// Forward the segment
	log.
		WithFields(logTags).
		WithField("source-id", segmentWrapper.Segment.SourceID).
		WithField("segment-name", segmentWrapper.Segment.Name).
		Debug("Sending out segment")
	err = f.client.ForwardSegment(f.workerCtxt, segmentWrapper.Segment)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", segmentWrapper.Segment.SourceID).
			WithField("segment-name", segmentWrapper.Segment.Name).
			Error("Failed to send out video segment")
		segmentWrapper.Feedback <- err
		return err
	}
	log.
		WithFields(logTags).
		WithField("source-id", segmentWrapper.Segment.SourceID).
		WithField("segment-name", segmentWrapper.Segment.Name).
		Debug("Video segment forwarded")
	segmentWrapper.Feedback <- nil
	return nil
}
