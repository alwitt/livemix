package edge

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/forwarder"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	"github.com/prometheus/client_golang/prometheus"
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

	// =====================================================================================
	// Utilities

	/*
		SyncActiveRecordingState sync the local recording states with that of system control node

			@param timestamp time.Time - current timestamp
	*/
	SyncActiveRecordingState(timestamp time.Time) error

	/*
		CacheEntryCount return the number of cached entries

			@param ctxt context.Context - execution context
			@returns the number of cached entries
	*/
	CacheEntryCount(ctxt context.Context) (int, error)
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
	worker           goutils.TaskProcessor
	wg               sync.WaitGroup
	workerCtxt       context.Context
	workerCtxtCancel context.CancelFunc
	rrClient         ControlRequestClient

	/* Support timers */
	reportTriggerTimer   goutils.IntervalTimer
	recordStateSyncTimer goutils.IntervalTimer

	/* Metrics Collection Agents */
	segmentReadMetrics     utils.SegmentMetricsAgent
	activeRecordingMetrics *prometheus.GaugeVec
}

/*
NewManager define a new video source operator

	@param parentCtxt context.Context - parent execution context
	@param params VideoSourceOperatorConfig - operator configuration
	@param params rrClient ControlRequestClient - request-response client for edge to call control
	@param metrics goutils.MetricsCollector - metrics framework client
	@return new operator
*/
func NewManager(
	parentCtxt context.Context,
	params VideoSourceOperatorConfig,
	rrClient ControlRequestClient,
	metrics goutils.MetricsCollector,
	tpMetrics goutils.TaskProcessorMetricHelper,
) (VideoSourceOperator, error) {
	logTags := log.Fields{
		"module": "edge", "component": "video-source-operator", "source-id": params.Self.ID,
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
		rrClient:                  rrClient,
		segmentReadMetrics:        nil,
		activeRecordingMetrics:    nil,
	}
	instance.workerCtxt, instance.workerCtxtCancel = context.WithCancel(parentCtxt)

	// -----------------------------------------------------------------------------
	// Prepare support timers

	// Define periodic timer for retrieving the current active recordings for video source
	recordingStateSyncLogTags := log.Fields{"sub-module": "recording-status-check-timer"}
	for lKey, lVal := range logTags {
		recordingStateSyncLogTags[lKey] = lVal
	}
	recordSyncTimer, err := goutils.GetIntervalTimerInstance(
		instance.workerCtxt, &instance.wg, recordingStateSyncLogTags,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define status report trigger timer")
		return nil, err
	}
	instance.recordStateSyncTimer = recordSyncTimer

	// Define periodic timer for sending video source status reports to system control node
	statusReportTimerLogTags := log.Fields{"sub-module": "status-report-timer"}
	for lKey, lVal := range logTags {
		statusReportTimerLogTags[lKey] = lVal
	}
	reportTriggerTimer, err := goutils.GetIntervalTimerInstance(
		instance.workerCtxt, &instance.wg, statusReportTimerLogTags,
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
		parentCtxt, "support-worker", 4, workerLogTags, tpMetrics,
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
	if err := worker.AddToTaskExecutionMap(
		reflect.TypeOf(inboundStopRecordingRequest{}), instance.stopRecording,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to install task definition")
		return nil, err
	}
	if err := worker.AddToTaskExecutionMap(
		reflect.TypeOf(common.VideoSegmentWithData{}), instance.newSegmentFromSource,
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

	// Start timer to periodically sync active recording sessions
	if rrClient != nil {
		err = recordSyncTimer.Start(params.StatusReportInterval, func() error {
			return instance.SyncActiveRecordingState(time.Now().UTC())
		}, false)
		if err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to start recording state sync timer")
			return nil, err
		}
	}

	// -----------------------------------------------------------------------------
	// Install metrics
	if metrics != nil {
		instance.segmentReadMetrics, err = utils.NewSegmentMetricsAgent(
			parentCtxt,
			metrics,
			utils.MetricsNameEdgeManagerSegmentReadLen,
			"Tracking total segment bytes read by edge manager",
			utils.MetricsNameEdgeManagerSegmentReadCount,
			"Tracking total segments read by edge manager",
			[]string{"source"},
		)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				Error("Unable to define segment IO tracking metrics helper agent")
			return nil, err
		}
		instance.activeRecordingMetrics, err = metrics.InstallCustomGaugeVecMetrics(
			parentCtxt,
			utils.MetricsNameEdgeManagerActiveRecordingCount,
			"Tracking currently active recording",
			[]string{"source"},
		)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				Error("Unable to define active recording count tracking metrics")
			return nil, err
		}
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
	if err := o.recordStateSyncTimer.Stop(); err != nil {
		return err
	}
	if err := o.worker.StopEventLoop(); err != nil {
		return err
	}
	if err := o.RecordingSegmentForwarder.Stop(ctxt); err != nil {
		return err
	}
	if err := o.LiveStreamSegmentForwarder.Stop(ctxt); err != nil {
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

	timestamp := time.Now().UTC()

	report := ipc.NewVideoSourceStatusReport(o.Self.ID, o.SelfReqRespTargetID, time.Now().UTC())

	if err := o.BroadcastClient.Broadcast(o.workerCtxt, &report); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to send status report message")
		return err
	}

	db := o.DBConns.NewPersistanceManager()
	defer db.Close()

	err := db.UpdateVideoSourceStats(o.workerCtxt, o.Self.ID, o.SelfReqRespTargetID, timestamp)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to locally update video source stats")
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
		WithField("req-source-id", newRecording.SourceID).
		WithField("recording-id", newRecording.ID).
		Debug("Submit 'start new recording request' for processing")

	if err := o.worker.Submit(ctxt, newRecording); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("req-source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			Error("Failed to submit 'start new recording request' for processing")
		return err
	}

	log.
		WithFields(logTags).
		WithField("req-source-id", newRecording.SourceID).
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

	// Force start time to UTC
	newRecording.StartTime = newRecording.StartTime.UTC()

	// Store the new recording session
	if err := dbClient.RecordKnownRecordingSession(o.workerCtxt, newRecording); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("req-source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			Error("Failed to record a known video recording")
		return err
	}

	log.
		WithFields(logTags).
		WithField("req-source-id", newRecording.SourceID).
		WithField("recording-id", newRecording.ID).
		Debug("Recorded a known video recording")

	// As the recording session may have started at an earlier point, any cached segments
	// which are fall under the recording window, is to be forwarded now

	relevantSegments, err := dbClient.ListAllLiveStreamSegmentsAfterTime(
		o.workerCtxt, newRecording.SourceID, newRecording.StartTime,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("req-source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			WithField("recording-start", newRecording.StartTime).
			Error("Failed to fetch segments associated with known recording")
		return err
	}

	{
		builder := strings.Builder{}
		builder.WriteString("[ ")
		for _, segment := range relevantSegments {
			builder.WriteString(
				fmt.Sprintf("(%s @ %s) ", segment.Name, segment.EndTime.Format(time.RFC3339)),
			)
		}
		builder.WriteString("]")
		log.
			WithFields(logTags).
			WithField("req-source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			WithField("recording-start", newRecording.StartTime.Format(time.RFC3339)).
			WithField("associated-segments", builder.String()).
			Debug("Found segments associated with known recording")
	}

	log.
		WithFields(logTags).
		WithField("req-source-id", newRecording.SourceID).
		WithField("recording-id", newRecording.ID).
		WithField("associated-segments", len(relevantSegments)).
		Info("Found segments associated with known recording")

	if len(relevantSegments) == 0 {
		return nil
	}

	// Fetch the segments content from cache
	segmentContents, err := o.VideoCache.GetSegments(o.workerCtxt, relevantSegments)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("req-source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			Error("Failed to fetch segment contents associated with known recording")
		dbClient.MarkExternalError(err)
		return err
	}

	log.
		WithFields(logTags).
		WithField("req-source-id", newRecording.SourceID).
		WithField("recording-id", newRecording.ID).
		WithField("cached-segments", len(segmentContents)).
		Debug("Found associated segment contents")

	// Combine the segment metadata with content
	fullRelevantSegments := []common.VideoSegmentWithData{}
	for _, aSegment := range relevantSegments {
		content, ok := segmentContents[aSegment.ID]
		if ok {
			fullRelevantSegments = append(fullRelevantSegments, common.VideoSegmentWithData{
				VideoSegment: aSegment, Content: content,
			})
		}
	}

	if len(fullRelevantSegments) > 0 {
		log.
			WithFields(logTags).
			WithField("req-source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			WithField("associated-segments", len(fullRelevantSegments)).
			Debug("Forwarding associated segments with content")

		// Forward the segments
		if err := o.RecordingSegmentForwarder.ForwardSegment(
			o.workerCtxt, []string{newRecording.ID}, fullRelevantSegments,
		); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("req-source-id", newRecording.SourceID).
				WithField("recording-id", newRecording.ID).
				Error("Failed to forward segment associated with known recording")
			dbClient.MarkExternalError(err)
			return err
		}

		log.
			WithFields(logTags).
			WithField("req-source-id", newRecording.SourceID).
			WithField("recording-id", newRecording.ID).
			WithField("associated-segments", len(fullRelevantSegments)).
			Debug("Associated segments with content forwarded")
	}

	return nil
}

// -------------------------------------------------------------------------------------
// Stop recording request

// inboundStopRecordingRequest inbound stop recording session request
type inboundStopRecordingRequest struct {
	RecordingID string
	EndTime     time.Time
}

func (o *videoSourceOperatorImpl) StopRecording(
	ctxt context.Context, recordingID string, endTime time.Time,
) error {
	logTags := o.GetLogTagsForContext(ctxt)

	log.
		WithFields(logTags).
		WithField("recording-id", recordingID).
		Debug("Submit 'stop recording request' for processing")

	request := inboundStopRecordingRequest{RecordingID: recordingID, EndTime: endTime}
	if err := o.worker.Submit(ctxt, request); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("recording-id", recordingID).
			Debug("Failed to submit 'stop recording request' for processing")
		return err
	}

	log.
		WithFields(logTags).
		WithField("recording-id", recordingID).
		Debug("'Stop recording request' submitted")

	return nil
}

func (o *videoSourceOperatorImpl) stopRecording(params interface{}) error {
	if request, ok := params.(inboundStopRecordingRequest); ok {
		return o.handleStopRecording(request)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := o.GetLogTagsForContext(o.workerCtxt)
	log.WithError(err).WithFields(logTags).Error("'StopRecording' processing failure")
	return err
}

func (o *videoSourceOperatorImpl) handleStopRecording(params inboundStopRecordingRequest) error {
	logTags := o.GetLogTagsForContext(o.workerCtxt)

	dbClient := o.DBConns.NewPersistanceManager()
	defer dbClient.Close()

	// Stop the recording session
	err := dbClient.MarkEndOfRecordingSession(o.workerCtxt, params.RecordingID, params.EndTime)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("recording-id", params.RecordingID).
			Error("Failed to mark video recording ended")
		return err
	}

	return nil
}

// =====================================================================================
// Video segment

func (o *videoSourceOperatorImpl) NewSegmentFromSource(
	ctxt context.Context, segment common.VideoSegmentWithData,
) error {
	logTags := o.GetLogTagsForContext(ctxt)

	log.
		WithFields(logTags).
		WithField("segment", segment.Name).
		Debug("Submit new segment from source for processing")

	if err := o.worker.Submit(ctxt, segment); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment", segment.Name).
			Error("Failed to submit new segment from source for processing")
		return err
	}

	log.
		WithFields(logTags).
		WithField("segment", segment.Name).
		Debug("New segment from source submitted")

	return nil
}

func (o *videoSourceOperatorImpl) newSegmentFromSource(params interface{}) error {
	if request, ok := params.(common.VideoSegmentWithData); ok {
		return o.handleNewSegmentFromSource(request)
	}
	err := fmt.Errorf("received unexpected call parameters: %s", reflect.TypeOf(params))
	logTags := o.GetLogTagsForContext(o.workerCtxt)
	log.WithError(err).WithFields(logTags).Error("'NewSegmentFromSource' processing failure")
	return err
}

func (o *videoSourceOperatorImpl) handleNewSegmentFromSource(
	segment common.VideoSegmentWithData,
) error {
	logTags := o.GetLogTagsForContext(o.workerCtxt)

	dbClient := o.DBConns.NewPersistanceManager()
	defer dbClient.Close()

	log.
		WithFields(logTags).
		WithField("segment", segment.Name).
		Debug("Submitting segment to live stream forwarder")

	// Forward to live stream segment forwarder
	if err := o.LiveStreamSegmentForwarder.ForwardSegment(o.workerCtxt, segment, false); err != nil {
		// even with errors proceed anyway
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment", segment.Name).
			Error("Unable to submit segment to live stream forwarder")
	}

	log.
		WithFields(logTags).
		WithField("segment", segment.Name).
		Debug("Segment submitted to live stream forwarder")

	// Find all active recordings at this time
	activeRecordings, err := dbClient.ListRecordingSessionsOfSource(o.workerCtxt, o.Self.ID, true)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment", segment.Name).
			Error("Failed to list active recordings")
		return err
	}

	// Update metrics
	if o.segmentReadMetrics != nil {
		o.segmentReadMetrics.RecordSegment(
			len(segment.Content), map[string]string{"source": segment.SourceID},
		)
	}
	if o.activeRecordingMetrics != nil {
		o.activeRecordingMetrics.
			With(prometheus.Labels{"source": segment.SourceID}).
			Set(float64(len(activeRecordings)))
	}

	if len(activeRecordings) == 0 {
		return nil
	}

	recordingIDs := []string{}
	for _, recording := range activeRecordings {
		recordingIDs = append(recordingIDs, recording.ID)
	}

	log.
		WithFields(logTags).
		WithField("recordings", len(recordingIDs)).
		WithField("segment", segment.Name).
		Debug("Submitting segment to recording forwarder")

	if err := o.RecordingSegmentForwarder.ForwardSegment(
		o.workerCtxt, recordingIDs, []common.VideoSegmentWithData{segment},
	); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("segment", segment.Name).
			Error("Unable to submit segment to recording forwarder")
		dbClient.MarkExternalError(err)
		return err
	}

	log.
		WithFields(logTags).
		WithField("recordings", len(recordingIDs)).
		WithField("segment", segment.Name).
		Debug("Segment submitted to recording forwarder")

	return nil
}

// =====================================================================================
// Utilities

func (o *videoSourceOperatorImpl) SyncActiveRecordingState(timestamp time.Time) error {
	logTags := o.GetLogTagsForContext(o.workerCtxt)

	// List currently active recordings
	activeRecordings, err := o.rrClient.ListActiveRecordingsOfSource(o.workerCtxt, o.Self.ID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to fetch list of active recordings from system control node")
		return err
	}

	{
		t, _ := json.Marshal(activeRecordings)
		log.
			WithFields(logTags).
			WithField("currently-active-recordings", string(t)).
			Debug("Associated active recordings reported by system control node")
	}

	// Get list of active recordings known locally
	dbClient := o.DBConns.NewPersistanceManager()
	defer dbClient.Close()

	knownActiveRecordings, err := dbClient.ListRecordingSessionsOfSource(o.workerCtxt, o.Self.ID, true)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to list of locally known active recordings")
		return err
	}

	activeRecordingByID := map[string]common.Recording{}
	for _, recording := range activeRecordings {
		activeRecordingByID[recording.ID] = recording
	}

	knownActiveRecordingByID := map[string]common.Recording{}
	for _, recording := range knownActiveRecordings {
		knownActiveRecordingByID[recording.ID] = recording
	}

	// ------------------------------------------------------------------------------------
	// Stop recording if system control node did not report recording as active

	for _, recording := range knownActiveRecordings {
		if _, ok := activeRecordingByID[recording.ID]; !ok {
			err := dbClient.MarkEndOfRecordingSession(o.workerCtxt, recording.ID, timestamp)
			if err != nil {
				log.
					WithError(err).
					WithFields(logTags).
					WithField("recording-id", recording.ID).
					Error("Failed to mark recording as complete")
				return err
			}
		}
	}

	// ------------------------------------------------------------------------------------
	// Start recording if active recording is not known locally

	for _, recording := range activeRecordings {
		if _, ok := knownActiveRecordingByID[recording.ID]; !ok {
			if err := o.StartRecording(o.workerCtxt, recording); err != nil {
				log.
					WithError(err).
					WithFields(logTags).
					WithField("recording-id", recording.ID).
					Error("Failed to install ongoing recording")
			}
		}
	}

	return nil
}

func (o *videoSourceOperatorImpl) CacheEntryCount(ctxt context.Context) (int, error) {
	return o.VideoCache.CacheEntryCount(ctxt)
}
