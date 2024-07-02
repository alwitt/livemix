package forwarder

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
)

// RecordingSegmentForwarder forward video segments associated with active recording sessions
type RecordingSegmentForwarder interface {
	/*
		Stop stop any background support tasks

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error

	/*
		ForwardSegment process new segments associated with active video recording sessions

			@param ctxt context.Context - execution context
			@param recordings []string - set of recording session IDs
			@param segments []common.VideoSegmentWithData - new video segments to report
	*/
	ForwardSegment(
		ctxt context.Context, recordings []string, segments []common.VideoSegmentWithData,
	) error
}

// s3RecordingSegmentForwarder S3 version of RecordingSegmentForwarder
type s3RecordingSegmentForwarder struct {
	goutils.Component
	storageCfg       common.RecordingStorageConfig
	s3Client         SegmentSender
	broadcastClient  utils.Broadcaster
	txWorkers        goutils.TaskProcessor
	wg               sync.WaitGroup
	workerCtxt       context.Context
	workerCtxtCancel context.CancelFunc
}

/*
NewS3RecordingSegmentForwarder define new S3 version of RecordingSegmentForwarder

	@param parentCtxt context.Context - forwarder's parent execution context
	@param storageCfg common.RecordingStorageConfig - segment storage config
	@param s3Client SegmentSender - S3 segment transport client
	@param broadcastClient utils.Broadcaster - message broadcast client
	@param maxInFlightSegments int - max number of segment being stored at any one time
	@param tpMetrics goutils.TaskProcessorMetricHelper - task processor metrics helper
	@returns new RecordingSegmentForwarder
*/
func NewS3RecordingSegmentForwarder(
	parentCtxt context.Context,
	storageCfg common.RecordingStorageConfig,
	s3Client SegmentSender,
	broadcastClient utils.Broadcaster,
	maxInFlightSegments int,
	tpMetrics goutils.TaskProcessorMetricHelper,
) (RecordingSegmentForwarder, error) {
	logTags := log.Fields{
		"module":    "forwarder",
		"component": "s3-recording-segment-forwarder",
	}

	instance := &s3RecordingSegmentForwarder{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		storageCfg:      storageCfg,
		s3Client:        s3Client,
		broadcastClient: broadcastClient,
		wg:              sync.WaitGroup{},
	}

	// Worker context
	instance.workerCtxt, instance.workerCtxtCancel = context.WithCancel(parentCtxt)

	// -----------------------------------------------------------------------------
	// Prepare TX workers
	txWorkerLogTags := log.Fields{
		"module":     "forwarder",
		"component":  "s3-recording-segment-forwarder",
		"sub-module": "transmit-worker",
	}
	workers, err := goutils.GetNewTaskDemuxProcessorInstance(
		instance.workerCtxt,
		"segment-forwarder-transmit",
		maxInFlightSegments+1,
		maxInFlightSegments+1,
		txWorkerLogTags,
		tpMetrics,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define transmission worker")
		return nil, err
	}
	instance.txWorkers = workers

	// Define support tasks
	if err := workers.AddToTaskExecutionMap(
		reflect.TypeOf(recordingSegmentForwardRequest{}), instance.forwardSegment,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to install task definition")
		return nil, err
	}

	// -----------------------------------------------------------------------------
	// Start the worker

	if err := workers.StartEventLoop(&instance.wg); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to start the transmission worker threads")
		return nil, err
	}

	return instance, nil
}

func (f *s3RecordingSegmentForwarder) Stop(ctxt context.Context) error {
	f.workerCtxtCancel()
	if err := f.txWorkers.StopEventLoop(); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &f.wg, time.Second*10)
}

// ======================================================================================
// Segment Processing

type recordingSegmentForwardRequest struct {
	recording []string
	segment   common.VideoSegmentWithData
}

func (f *s3RecordingSegmentForwarder) ForwardSegment(
	ctxt context.Context, recordings []string, segments []common.VideoSegmentWithData,
) error {
	logTags := f.GetLogTagsForContext(ctxt)

	// Process each segment separately
	requests := []recordingSegmentForwardRequest{}
	for _, oneSegment := range segments {
		// Define segment storage URI. This tells the sender where to place the object
		oneSegment.URI = fmt.Sprintf(
			"s3://%s/%s/%s/%s",
			f.storageCfg.StorageBucket,
			f.storageCfg.StorageObjectPrefix,
			oneSegment.SourceID,
			oneSegment.Name,
		)
		log.
			WithFields(logTags).
			WithField("source-id", oneSegment.SourceID).
			WithField("segment", oneSegment.Name).
			WithField("storage-uri", oneSegment.URI).
			Debug("Newly defined recording segment storage URI")

		requests = append(requests, recordingSegmentForwardRequest{
			recording: recordings,
			segment:   oneSegment,
		})
	}

	log.
		WithFields(logTags).
		WithField("segments-to-record", len(segments)).
		Debug("Submitting new recording segment forward requests")

	for _, request := range requests {
		if err := f.txWorkers.Submit(ctxt, request); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source-id", request.segment.SourceID).
				WithField("segment", request.segment.Name).
				Error("Failed to submit a new recording segment forward request")
		}
		// Continue even if one fails.
	}

	log.
		WithFields(logTags).
		WithField("segments-to-record", len(segments)).
		Debug("Processed all new recording segment forward requests")

	return nil
}

func (f *s3RecordingSegmentForwarder) forwardSegment(params interface{}) error {
	if request, ok := params.(recordingSegmentForwardRequest); ok {
		return f.handleForwardSegment(request)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := f.GetLogTagsForContext(f.workerCtxt)
	log.WithError(err).WithFields(logTags).Error("'ForwardSegment' processing failure")
	return err
}

func (f *s3RecordingSegmentForwarder) handleForwardSegment(
	param recordingSegmentForwardRequest,
) error {
	logTags := f.GetLogTagsForContext(f.workerCtxt)

	if param.segment.Uploaded == nil {
		log.
			WithFields(logTags).
			WithField("source-id", param.segment.SourceID).
			WithField("segment", param.segment.Name).
			WithField("storage-uri", param.segment.URI).
			Debug("Storing recording segment")

		// Forward the segment
		if err := f.s3Client.ForwardSegment(f.workerCtxt, param.segment); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source-id", param.segment.SourceID).
				WithField("segment", param.segment.Name).
				WithField("storage-uri", param.segment.URI).
				Error("Unable to forward recording segment")
			return err
		}

		log.
			WithFields(logTags).
			WithField("source-id", param.segment.SourceID).
			WithField("segment", param.segment.Name).
			WithField("storage-uri", param.segment.URI).
			Debug("Recording segment stored")
	}

	// Send a broadcast that this segment has been forwarded to storage
	report := ipc.NewRecordingSegmentReport(
		param.recording, []common.VideoSegment{param.segment.VideoSegment},
	)
	if err := f.broadcastClient.Broadcast(f.workerCtxt, &report); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", param.segment.SourceID).
			WithField("segment", param.segment.Name).
			WithField("storage-uri", param.segment.URI).
			Error("Unable to broadcast recording segment stored event")
		return err
	}

	return nil
}
