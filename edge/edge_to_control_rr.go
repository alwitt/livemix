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

/*

TODO FIXME:

* Support inbound request to notify an edge node to start a recording
* Support inbound request to notify an edge node to stop a recording

*/

// ControlRequestClient request-response client for edge to call control
type ControlRequestClient interface {
	/*
		InstallReferenceToManager used by a VideoSourceOperator to add a reference of itself
		into the client

			@param newManager VideoSourceOperator - reference to the manager
	*/
	InstallReferenceToManager(newManager VideoSourceOperator)

	/*
		GetVideoSourceInfo query control for a video source's information

			@param ctxt context.Context - execution context
			@param sourceName string - video source name
			@returns video source info
	*/
	GetVideoSourceInfo(ctxt context.Context, sourceName string) (common.VideoSource, error)

	/*
		StopAllAssociatedRecordings request all recording associated this this source is stopped

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
	*/
	StopAllAssociatedRecordings(ctxt context.Context, sourceID string) error
}

// controlRequestClientImpl implements ControlRequestClient
type controlRequestClientImpl struct {
	ipc.RequestResponseDriver
	core              VideoSourceOperator
	controlRRTargetID string
	requestTimeout    time.Duration
	validator         *validator.Validate
}

/*
NewControlRequestClient define a new edge to control request-response client

	@param ctxt context.Context - execution context
	@param clientName string - name of this client instance
	@param controlRRTargetID string - control's target ID for request-response targeting
	@param coreClient goutils.RequestResponseClient - core request-response client
	@param requestTimeout time.Duration - request-response request timeout
	@returns new client
*/
func NewControlRequestClient(
	ctxt context.Context,
	clientName string,
	controlRRTargetID string,
	coreClient goutils.RequestResponseClient,
	requestTimeout time.Duration,
) (ControlRequestClient, error) {
	logTags := log.Fields{
		"module": "api", "component": "edge-to-control-rr-client", "instance": clientName,
	}

	instance := &controlRequestClientImpl{
		RequestResponseDriver: ipc.RequestResponseDriver{
			Component: goutils.Component{
				LogTags: logTags,
				LogTagModifiers: []goutils.LogMetadataModifier{
					goutils.ModifyLogMetadataByRestRequestParam,
				},
			},
			Client: coreClient,
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

	return instance, nil
}

func (c *controlRequestClientImpl) InstallReferenceToManager(newManager VideoSourceOperator) {
	c.core = newManager
}

// ======================================================================================
// Inbound Request Processing

func (c *controlRequestClientImpl) processInboundStreamingStateChangeRequest(
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
		return nil, fmt.Errorf("no reference to EdgeManager set yet")
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

// ======================================================================================
// Outbound Request Processing

func (c *controlRequestClientImpl) GetVideoSourceInfo(
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
		return common.VideoSource{}, fmt.Errorf(response.ErrorMsg)

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

func (c *controlRequestClientImpl) StopAllAssociatedRecordings(
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
			return fmt.Errorf(response.ErrorMsg)
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
