package edge

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

// pubSubControlRequestClientImpl implements ControlRequestClient
type pubSubControlRequestClientImpl struct {
	goutils.RequestResponseDriver
	core              VideoSourceOperator
	controlRRTargetID string
	requestTimeout    time.Duration
	validator         *validator.Validate
}

/*
NewPubSubControlRequestClient define a new edge to control request-response client based on PubSub

	@param ctxt context.Context - execution context
	@param clientName string - name of this client instance
	@param controlRRTargetID string - control's target ID for request-response targeting
	@param coreClient goutils.RequestResponseClient - core request-response client
	@param requestTimeout time.Duration - request-response request timeout
	@returns new client
*/
func NewPubSubControlRequestClient(
	ctxt context.Context,
	clientName string,
	controlRRTargetID string,
	coreClient goutils.RequestResponseClient,
	requestTimeout time.Duration,
) (ControlRequestClient, error) {
	logTags := log.Fields{
		"module": "api", "component": "edge-to-control-rr-client", "instance": clientName,
	}

	instance := &pubSubControlRequestClientImpl{
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
		core:              nil,
		controlRRTargetID: controlRRTargetID,
		requestTimeout:    requestTimeout,
		validator:         validator.New(),
	}

	// Install inbound request handling
	if err := coreClient.SetInboundRequestHandler(ctxt, instance.ProcessInboundRequest); err != nil {
		return nil, err
	}
	instance.InstallHandler(
		reflect.TypeOf(ipc.ChangeSourceStreamingStateRequest{}),
		instance.processInboundStreamingStateChangeRequest,
	)
	instance.InstallHandler(
		reflect.TypeOf(ipc.StartVideoRecordingRequest{}),
		instance.processInboundStartNewRecordingRequest,
	)
	instance.InstallHandler(
		reflect.TypeOf(ipc.StopVideoRecordingRequest{}),
		instance.processInboundStopRecordingRequest,
	)

	return instance, nil
}

func (c *pubSubControlRequestClientImpl) InstallReferenceToManager(newManager VideoSourceOperator) {
	c.core = newManager
}

// ======================================================================================
// Inbound Request Processing

// --------------------------------------------------------------------------------------
// Process inbound streaming state change request

func (c *pubSubControlRequestClientImpl) processInboundStreamingStateChangeRequest(
	ctxt context.Context,
	requestRaw interface{},
	origMsg goutils.ReqRespMessage,
) (interface{}, error) {
	logTag := c.GetLogTagsForContext(ctxt)

	request, ok := requestRaw.(ipc.ChangeSourceStreamingStateRequest)
	if !ok {
		return nil, fmt.Errorf(
			"incorrect request type '%s' for 'change streaming state'", reflect.TypeOf(requestRaw),
		)
	}

	if c.core == nil {
		return nil, fmt.Errorf("no reference to VideoSourceOperator set yet")
	}

	err := c.core.ChangeVideoSourceStreamState(ctxt, request.SourceID, request.NewState)
	var response ipc.GeneralResponse
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", origMsg.SenderID).
			WithField("request-id", origMsg.RequestID).
			Errorf("Failed to change source '%s' streaming state", request.SourceID)
		response = ipc.NewGeneralResponse(false, err.Error())
	} else {
		response = ipc.NewGeneralResponse(true, "")
	}

	return response, nil
}

// --------------------------------------------------------------------------------------
// Process inbound recording start request

func (c *pubSubControlRequestClientImpl) processInboundStartNewRecordingRequest(
	ctxt context.Context,
	requestRaw interface{},
	origMsg goutils.ReqRespMessage,
) (interface{}, error) {
	logTag := c.GetLogTagsForContext(ctxt)

	request, ok := requestRaw.(ipc.StartVideoRecordingRequest)
	if !ok {
		return nil, fmt.Errorf(
			"incorrect request type '%s' for 'start recording'", reflect.TypeOf(requestRaw),
		)
	}

	if c.core == nil {
		return nil, fmt.Errorf("no reference to VideoSourceOperator set yet")
	}

	err := c.core.StartRecording(ctxt, request.Session)
	var response ipc.GeneralResponse
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", origMsg.SenderID).
			WithField("request-id", origMsg.RequestID).
			Errorf("Start new recording '%s' locally failed", request.Session.ID)
		response = ipc.NewGeneralResponse(false, err.Error())
	} else {
		response = ipc.NewGeneralResponse(true, "")
	}

	return response, nil
}

// --------------------------------------------------------------------------------------
// Process inbound recording stop request

func (c *pubSubControlRequestClientImpl) processInboundStopRecordingRequest(
	ctxt context.Context,
	requestRaw interface{},
	origMsg goutils.ReqRespMessage,
) (interface{}, error) {
	logTag := c.GetLogTagsForContext(ctxt)

	request, ok := requestRaw.(ipc.StopVideoRecordingRequest)
	if !ok {
		return nil, fmt.Errorf(
			"incorrect request type '%s' for 'stop recording'", reflect.TypeOf(requestRaw),
		)
	}

	if c.core == nil {
		return nil, fmt.Errorf("no reference to VideoSourceOperator set yet")
	}

	err := c.core.StopRecording(ctxt, request.RecordingID, request.EndTime)
	var response ipc.GeneralResponse
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", origMsg.SenderID).
			WithField("request-id", origMsg.RequestID).
			Errorf("Stop recording '%s' locally failed", request.RecordingID)
		response = ipc.NewGeneralResponse(false, err.Error())
	} else {
		response = ipc.NewGeneralResponse(true, "")
	}

	return response, nil
}

// ======================================================================================
// Outbound Request Processing

func (c *pubSubControlRequestClientImpl) GetVideoSourceInfo(
	ctxt context.Context, sourceName string,
) (common.VideoSource, error) {
	logTags := c.GetLogTagsForContext(ctxt)

	// Build the request
	msg := ipc.NewGetVideoSourceByNameRequest(sourceName)
	msgStr, err := json.Marshal(&msg)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Fail to build video source info request IPC msg")
		return common.VideoSource{}, nil
	}

	// Prepare the request parameters
	callParam := goutils.RequestCallParam{
		ExpectedResponsesCount: 1,
		Blocking:               false,
		Timeout:                c.requestTimeout,
	}

	// Make request
	results, err := c.MakeRequest(
		ctxt,
		fmt.Sprintf("Fetch video source '%s' info", sourceName),
		c.controlRRTargetID,
		msgStr,
		nil,
		callParam,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("video-source", sourceName).
			Error("Video source info request failed")
		return common.VideoSource{}, err
	}

	// Process the responses
	answer := results[0]
	switch reflect.TypeOf(answer) {
	case reflect.TypeOf(ipc.GetVideoSourceByNameResponse{}):
		videoInfo := answer.(ipc.GetVideoSourceByNameResponse)
		if err := c.validator.Struct(&videoInfo); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				Error("Can't process invalid 'GetVideoSourceByNameResponse' from control")
			return common.VideoSource{}, err
		}
		return videoInfo.Source, nil

	case reflect.TypeOf(ipc.GeneralResponse{}):
		response := answer.(ipc.GeneralResponse)
		return common.VideoSource{}, fmt.Errorf("%s", response.ErrorMsg)

	default:
		err := fmt.Errorf("unknown supported response type '%s'", reflect.TypeOf(answer))
		log.
			WithError(err).
			WithFields(logTags).
			WithField("video-source", sourceName).
			Error("Unable to parse video source info response")
		return common.VideoSource{}, err
	}
}

func (c *pubSubControlRequestClientImpl) ListActiveRecordingsOfSource(
	ctxt context.Context, sourceID string,
) ([]common.Recording, error) {
	logTags := c.GetLogTagsForContext(ctxt)

	// Build the request
	msg := ipc.NewListActiveRecordingsRequest(sourceID)
	msgStr, err := json.Marshal(&msg)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to build list all active recordings of source request IPC msg")
		return nil, err
	}

	// Prepare the request parameters
	callParam := goutils.RequestCallParam{
		ExpectedResponsesCount: 1,
		Blocking:               false,
		Timeout:                c.requestTimeout,
	}

	// Make request
	results, err := c.MakeRequest(
		ctxt,
		fmt.Sprintf("Get active recordings of video source '%s'", sourceID),
		c.controlRRTargetID,
		msgStr,
		nil,
		callParam,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			Error("Get active recordings of video source request failed")
		return nil, err
	}

	// Process the responses
	answer := results[0]
	switch reflect.TypeOf(answer) {
	case reflect.TypeOf(ipc.GeneralResponse{}):
		response := answer.(ipc.GeneralResponse)
		return nil, fmt.Errorf("%s", response.ErrorMsg)

	case reflect.TypeOf(ipc.ListActiveRecordingsResponse{}):
		response := answer.(ipc.ListActiveRecordingsResponse)
		return response.Recordings, nil

	default:
		err := fmt.Errorf("unknown supported response type '%s'", reflect.TypeOf(answer))
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			Error("Unable to parse list active recording sessions response")
		return nil, err
	}
}

func (c *pubSubControlRequestClientImpl) StopAllAssociatedRecordings(
	ctxt context.Context, sourceID string,
) error {
	logTags := c.GetLogTagsForContext(ctxt)

	// Build the request
	msg := ipc.NewCloseAllActiveRecordingRequest(sourceID)
	msgStr, err := json.Marshal(&msg)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to build stop all associated recording session request IPC msg")
		return err
	}

	// Prepare the request parameters
	callParam := goutils.RequestCallParam{
		ExpectedResponsesCount: 1,
		Blocking:               false,
		Timeout:                c.requestTimeout,
	}

	// Make request
	results, err := c.MakeRequest(
		ctxt,
		fmt.Sprintf("Stop all video source '%s' recording sessions", sourceID),
		c.controlRRTargetID,
		msgStr,
		nil,
		callParam,
	)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			Error("Stop all recording session of source request failed")
		return err
	}

	// Process the responses
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
			WithField("source-id", sourceID).
			Error("Unable to parse stop all recording response")
		return err
	}
}

func (c *pubSubControlRequestClientImpl) ExchangeVideoSourceStatus(
	ctxt context.Context, sourceID string, reqRespTargetID string, localTime time.Time,
) (common.VideoSource, []common.Recording, error) {
	return common.VideoSource{}, nil, nil
}
