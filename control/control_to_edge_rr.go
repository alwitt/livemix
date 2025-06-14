package control

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
)

// EdgeRequestClient request-response client for control to call edge
type EdgeRequestClient interface {
	/*
		InstallReferenceToManager used by a SystemManager to add a reference of itself into the client

			@param newManager SystemManager - reference to the manager
	*/
	InstallReferenceToManager(newManager SystemManager)

	/*
		ChangeVideoStreamingState change a video source's streaming state

			@param ctxt context.Context - execution context
			@param source common.VideoSource - video source entry
			@param newState int - new streaming state
	*/
	ChangeVideoStreamingState(ctxt context.Context, source common.VideoSource, newState int) error

	/*
		StartRecordingSession start a new recording session on a edge node

			@param ctxt context.Context - execution context
			@param source common.VideoSource - video source entry
			@param recording common.Recording - video recording session
	*/
	StartRecordingSession(
		ctxt context.Context, source common.VideoSource, recording common.Recording,
	) error

	/*
		StopRecordingSession stop a recording session on a edge node

			@param ctxt context.Context - execution context
			@param source common.VideoSource - video source entry
			@param recordingID string - video recording session ID
	*/
	StopRecordingSession(
		ctxt context.Context, source common.VideoSource, recordingID string, endTime time.Time,
	) error
}

// edgeRequestClientImpl implements EdgeRequestClient
type edgeRequestClientImpl struct {
	goutils.RequestResponseDriver
	core           SystemManager
	requestTimeout time.Duration
	validator      *validator.Validate
}

/*
NewEdgeRequestClient define a new control to edge request-response client

	@param ctxt context.Context - execution context
	@param clientName string - name of this client instance
	@param coreClient goutils.RequestResponseClient - core request-response client
	@param requestTimeout time.Duration - request-response request timeout
	@returns new client
*/
func NewEdgeRequestClient(
	ctxt context.Context,
	clientName string,
	coreClient goutils.RequestResponseClient,
	requestTimeout time.Duration,
) (EdgeRequestClient, error) {
	logTags := log.Fields{
		"module": "api", "component": "control-to-edge-rr-client", "instance": clientName,
	}

	instance := &edgeRequestClientImpl{
		RequestResponseDriver: goutils.RequestResponseDriver{
			Component: goutils.Component{
				LogTags: logTags,
				LogTagModifiers: []goutils.LogMetadataModifier{
					goutils.ModifyLogMetadataByRestRequestParam,
				},
			},
			Client:        coreClient,
			PayloadParser: ipc.ParseRawMessage,
		},
		core:           nil,
		requestTimeout: requestTimeout,
		validator:      validator.New(),
	}

	// Install inbound request handling
	if err := coreClient.SetInboundRequestHandler(ctxt, instance.ProcessInboundRequest); err != nil {
		return nil, err
	}
	instance.InstallHandler(
		reflect.TypeOf(ipc.GetVideoSourceByNameRequest{}),
		instance.processInboundVideoSourceInfoRequest,
	)
	instance.InstallHandler(
		reflect.TypeOf(ipc.ListActiveRecordingsRequest{}),
		instance.processInboundActiveRecordingsOfSource,
	)

	return instance, nil
}

func (c *edgeRequestClientImpl) InstallReferenceToManager(newManager SystemManager) {
	c.core = newManager
}

// ======================================================================================
// Inbound Request Processing

func (c *edgeRequestClientImpl) processInboundVideoSourceInfoRequest(
	ctxt context.Context, requestRaw interface{}, origMsg goutils.ReqRespMessage,
) (interface{}, error) {
	logTag := c.GetLogTagsForContext(ctxt)

	request, ok := requestRaw.(ipc.GetVideoSourceByNameRequest)
	if !ok {
		return nil, fmt.Errorf(
			"incorrect request type '%s' for 'get video source info'", reflect.TypeOf(requestRaw),
		)
	}

	if c.core == nil {
		err := fmt.Errorf("no reference to SystemManager set yet")
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", origMsg.SenderID).
			WithField("request-id", origMsg.RequestID).
			Error("Unable to start request handling")
		return nil, err
	}

	// Process video source info request
	sourceInfo, err := c.core.GetVideoSourceByName(ctxt, request.TargetName)
	var response interface{}
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", origMsg.SenderID).
			WithField("request-id", origMsg.RequestID).
			Errorf("Failed to read video source '%s' info", request.TargetName)
		response = ipc.NewGeneralResponse(false, err.Error())
	} else {
		response = ipc.NewGetVideoSourceByNameResponse(sourceInfo)
	}

	return response, nil
}

func (c *edgeRequestClientImpl) processInboundActiveRecordingsOfSource(
	ctxt context.Context, requestRaw interface{}, origMsg goutils.ReqRespMessage,
) (interface{}, error) {
	logTag := c.GetLogTagsForContext(ctxt)

	request, ok := requestRaw.(ipc.ListActiveRecordingsRequest)
	if !ok {
		return nil, fmt.Errorf(
			"incorrect request type '%s' for 'list active recordings of source'",
			reflect.TypeOf(requestRaw),
		)
	}

	if c.core == nil {
		err := fmt.Errorf("no reference to SystemManager set yet")
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", origMsg.SenderID).
			WithField("request-id", origMsg.RequestID).
			Error("Unable to start request handling")
		return nil, err
	}

	// List active recording source
	recordings, err := c.core.ListRecordingSessionsOfSource(ctxt, request.SourceID, true)
	var response interface{}
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", origMsg.SenderID).
			WithField("request-id", origMsg.RequestID).
			Errorf("Failed to get active recording of source '%s'", request.SourceID)
		response = ipc.NewGeneralResponse(false, err.Error())
	} else {
		response = ipc.NewListActiveRecordingsResponse(recordings)
	}

	return response, nil
}

// ======================================================================================
// Outbound Request Processing

// basicRequestResponse helper function to standardize the RR process where only
// generic responses are expected from one responder
func (c *edgeRequestClientImpl) basicRequestResponse(
	ctxt context.Context, targetID, requestInstanceName string, msg []byte,
) error {
	logTags := c.GetLogTagsForContext(ctxt)

	// Prepare the request parameters
	callParam := goutils.RequestCallParam{
		ExpectedResponsesCount: 1,
		Blocking:               false,
		Timeout:                c.requestTimeout,
	}

	// Make request
	results, err := c.MakeRequest(ctxt, requestInstanceName, targetID, msg, nil, callParam)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("request-instance", requestInstanceName).
			Error("Failed to make request")
		return err
	}

	// Process the response
	answer := results[0]
	switch reflect.TypeOf(answer) {
	case reflect.TypeOf(ipc.GeneralResponse{}):
		response := answer.(ipc.GeneralResponse)
		if !response.Success {
			return fmt.Errorf("%s", response.ErrorMsg)
		}
		return nil

	default:
		err := fmt.Errorf("unknown supported response type '%s'", reflect.TypeOf(answer))
		log.
			WithError(err).
			WithFields(logTags).
			WithField("request-instance", requestInstanceName).
			Errorf("Unable to parse response")
		return err
	}
}

func (c *edgeRequestClientImpl) ChangeVideoStreamingState(
	ctxt context.Context, source common.VideoSource, newState int,
) error {
	logTags := c.GetLogTagsForContext(ctxt)

	if source.ReqRespTargetID == nil {
		err := fmt.Errorf("video source have not reported a request-response target ID")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", source.ID).
			Error("Unable to make request")
		return err
	}

	// BUild the request
	msg := ipc.NewChangeSourceStreamingStateRequest(source.ID, newState)
	msgStr, err := json.Marshal(&msg)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Fail to build streaming state change IPC msg")
		return nil
	}

	return c.basicRequestResponse(
		ctxt,
		*source.ReqRespTargetID,
		fmt.Sprintf("Change video source '%s' streaming state", source.Name),
		msgStr,
	)
}

func (c *edgeRequestClientImpl) StartRecordingSession(
	ctxt context.Context, source common.VideoSource, recording common.Recording,
) error {
	logTags := c.GetLogTagsForContext(ctxt)

	if source.ReqRespTargetID == nil {
		err := fmt.Errorf("video source have not reported a request-response target ID")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", source.ID).
			Error("Unable to make request")
		return err
	}

	// Build the request
	msg := ipc.NewStartVideoRecordingSessionRequest(recording)
	msgStr, err := json.Marshal(&msg)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Fail to build start video recording IPC msg")
		return nil
	}

	return c.basicRequestResponse(
		ctxt,
		*source.ReqRespTargetID,
		fmt.Sprintf(
			"Start recording session '%s' on video source '%s'", recording.ID, source.Name,
		),
		msgStr,
	)
}

func (c *edgeRequestClientImpl) StopRecordingSession(
	ctxt context.Context, source common.VideoSource, recordingID string, endTime time.Time,
) error {
	logTags := c.GetLogTagsForContext(ctxt)

	if source.ReqRespTargetID == nil {
		err := fmt.Errorf("video source have not reported a request-response target ID")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", source.ID).
			Error("Unable to make request")
		return err
	}

	// Build the request
	msg := ipc.NewStopVideoRecordingSessionRequest(recordingID, endTime)
	msgStr, err := json.Marshal(&msg)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Fail to build stop video recording IPC msg")
		return nil
	}

	return c.basicRequestResponse(
		ctxt,
		*source.ReqRespTargetID,
		fmt.Sprintf("Stop recording session '%s' on video source '%s'", recordingID, source.Name),
		msgStr,
	)
}
