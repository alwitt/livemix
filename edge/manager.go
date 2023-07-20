package edge

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/forwarder"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
)

// VideoSourceOperator video source operations manager
type VideoSourceOperator interface {
	/*
		Ready check whether the DB connection is working

			@param ctxt context.Context - execution context
	*/
	Ready(ctxt context.Context) error

	/*
		Stop stop any support background tasks which were started

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error

	// =====================================================================================
	// Video sources

	/*
		RecordKnownVideoSource create record for a known video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
			@param name string - source name
			@param segmentLen int - target segment length in secs
			@param playlistURI *string - video source playlist URI
			@param description *string - optionally, source description
			@param streaming int - whether the video source is currently streaming
	*/
	RecordKnownVideoSource(
		ctxt context.Context,
		id, name string,
		segmentLen int,
		playlistURI, description *string,
		streaming int,
	) error

	/*
		ChangeVideoSourceStreamState change the streaming state for a video source

			@param ctxt context.Context - execution context
			@param id string - source ID
			@param streaming int - new streaming state
	*/
	ChangeVideoSourceStreamState(ctxt context.Context, id string, streaming int) error

	// =====================================================================================
	// Video recording sessions

	/*
		StartRecording process new recording request

			@param ctxt context.Context - execution context
			@param newRecording common.Recording - new recording session to be started
	*/
	StartRecording(ctxt context.Context, newRecording common.Recording) error

	/*
		StopRecording process recording stop request

			@param ctxt context.Context - execution context
			@param recordingID string - recording session ID
			@param endTime time.Time
	*/
	StopRecording(ctxt context.Context, recordingID string, endTime time.Time) error

	// =====================================================================================
	// Video segment

	/*
		NewSegmentFromSource process new video segment produced by a video source

			@param ctxt context.Context - execution context
			@param segment common.VideoSegmentWithData - the new video segment
	*/
	NewSegmentFromSource(ctxt context.Context, segment common.VideoSegmentWithData) error
}

// VideoSourceOperatorConfig video source operations manager configuration
type VideoSourceOperatorConfig struct {
	// Self a reference to the video source being managed
	Self common.VideoSource
	// SelfReqRespTargetID the request-response target ID to send inbound request to this operator
	SelfReqRespTargetID string
	// DBConns DB connection manager
	DBConns db.ConnectionManager
	// VideoCache video segment cache
	VideoCache utils.VideoSegmentCache
	// BroadcastClient message broadcast client
	BroadcastClient utils.Broadcaster
	// RecordingSegmentForwarder client for forwarding segments associated with recording sessions
	RecordingSegmentForwarder forwarder.RecordingSegmentForwarder
	// LiveStreamSegmentForwarder client for forwarding segments associated with live stream
	LiveStreamSegmentForwarder forwarder.LiveStreamSegmentForwarder
	// StatusReportInterval status report interval
	StatusReportInterval time.Duration
}

// videoSourceOperatorImpl implements VideoSourceOperator
type videoSourceOperatorImpl struct {
	goutils.Component
	VideoSourceOperatorConfig
	reportTriggerTimer goutils.IntervalTimer
	worker             goutils.TaskProcessor
	wg                 sync.WaitGroup
	workerCtxt         context.Context
	workerCtxtCancel   context.CancelFunc
}

/*
NewManager define a new video source operator

	@param parentCtxt context.Context - parent execution context
	@param params VideoSourceOperatorConfig - operator configuration
	@return new operator
*/
func NewManager(
	parentCtxt context.Context, params VideoSourceOperatorConfig,
) (VideoSourceOperator, error) {
	logTags := log.Fields{
		"module": "edge", "component": "video-source-operator", "instance": params.Self.Name,
	}

	instance := &videoSourceOperatorImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		VideoSourceOperatorConfig: params,
		wg:                        sync.WaitGroup{},
	}
	instance.workerCtxt, instance.workerCtxtCancel = context.WithCancel(parentCtxt)

	// Define periodic timer for sending video source status reports to system control node
	timerLogTags := log.Fields{"sub-module": "status-report-timer"}
	for lKey, lVal := range logTags {
		timerLogTags[lKey] = lVal
	}
	reportTriggerTimer, err := goutils.GetIntervalTimerInstance(
		instance.workerCtxt, &instance.wg, timerLogTags,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define status report trigger timer")
		return nil, err
	}
	instance.reportTriggerTimer = reportTriggerTimer

	// Make first status report
	if err := instance.sendSourceStatusReport(); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to send first status report")
		return nil, err
	}

	// -----------------------------------------------------------------------------
	// Prepare worker
	workerLogTags := log.Fields{"sub-module": "support-worker"}
	for lKey, lVal := range logTags {
		workerLogTags[lKey] = lVal
	}
	worker, err := goutils.GetNewTaskProcessorInstance(
		parentCtxt, "support-worker", 4, workerLogTags,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define support worker")
		return nil, err
	}
	instance.worker = worker

	// Define support tasks
	if err := worker.AddToTaskExecutionMap(
		reflect.TypeOf(common.Recording{}), instance.startRecording,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to install task definition")
		return nil, err
	}

	// -----------------------------------------------------------------------------
	// Start the worker
	if err = worker.StartEventLoop(&instance.wg); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to start support worker")
		return nil, err
	}

	// Start timer to periodically send video source status reports to the system control node
	err = reportTriggerTimer.Start(
		params.StatusReportInterval, instance.sendSourceStatusReport, false,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to start status report trigger timer")
		return nil, err
	}

	return instance, nil
}

func (o *videoSourceOperatorImpl) Ready(ctxt context.Context) error {
	dbClient := o.DBConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.Ready(ctxt)
}

func (o *videoSourceOperatorImpl) Stop(ctxt context.Context) error {
	o.workerCtxtCancel()
	if err := o.reportTriggerTimer.Stop(); err != nil {
		return err
	}
	if err := o.worker.StopEventLoop(); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &o.wg, time.Second*5)
}

// =====================================================================================
// Video sources

func (o *videoSourceOperatorImpl) RecordKnownVideoSource(
	ctxt context.Context,
	id, name string,
	segmentLen int,
	playlistURI, description *string,
	streaming int,
) error {
	dbClient := o.DBConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.RecordKnownVideoSource(
		ctxt, o.Self.ID, name, segmentLen, playlistURI, description, streaming,
	)
}

func (o *videoSourceOperatorImpl) ChangeVideoSourceStreamState(
	ctxt context.Context, id string, streaming int,
) error {
	dbClient := o.DBConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.ChangeVideoSourceStreamState(ctxt, o.Self.ID, streaming)
}

func (o *videoSourceOperatorImpl) sendSourceStatusReport() error {
	logTags := o.GetLogTagsForContext(o.workerCtxt)

	report := ipc.NewVideoSourceStatusReport(o.Self.ID, o.SelfReqRespTargetID, time.Now().UTC())

	if err := o.BroadcastClient.Broadcast(o.workerCtxt, &report); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to send status report message")
		return err
	}

	return nil
}

// =====================================================================================
// Video recording sessions

// -------------------------------------------------------------------------------------
// Start recording request

func (o *videoSourceOperatorImpl) StartRecording(
	ctxt context.Context, newRecording common.Recording,
) error {
	logTags := o.GetLogTagsForContext(ctxt)

	log.
		WithFields(logTags).
		WithField("source-id", newRecording.SourceID).
		WithField("recording-id", newRecording.ID).
		Debug("Submit 'start new recording request' for processing")

	if err := o.worker.Submit(ctxt, newRecording); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			Error("Failed to submit 'start new recording request' for processing")
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", newRecording.SourceID).
		WithField("recording-id", newRecording.ID).
		Debug("'Start new recording request' submitted")

	return nil
}

func (o *videoSourceOperatorImpl) startRecording(params interface{}) error {
	if request, ok := params.(common.Recording); ok {
		return o.handleStartRecording(request)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := o.GetLogTagsForContext(o.workerCtxt)
	log.WithError(err).WithFields(logTags).Error("'StartRecording' processing failure")
	return err
}

func (o *videoSourceOperatorImpl) handleStartRecording(newRecording common.Recording) error {
	logTags := o.GetLogTagsForContext(o.workerCtxt)

	dbClient := o.DBConns.NewPersistanceManager()
	defer dbClient.Close()

	// Store the new recording session
	if err := dbClient.RecordKnownRecordingSession(o.workerCtxt, newRecording); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			Error("Failed to record a known video recording")
		return err
	}

	// As the recording session may have started before

	return nil
}

// -------------------------------------------------------------------------------------
// Stop recording request

func (o *videoSourceOperatorImpl) StopRecording(
	ctxt context.Context, recordingID string, endTime time.Time,
) error {
	return nil
}

// =====================================================================================
// Video segment

func (o *videoSourceOperatorImpl) NewSegmentFromSource(
	ctxt context.Context, segment common.VideoSegmentWithData,
) error {
	return nil
}
