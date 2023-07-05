package control

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/alwitt/goutils"
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
			@param sourceID string - source ID
			@param newState bool - new streaming state
	*/
	ChangeVideoStreamingState(ctxt context.Context, sourceID string, newState bool) error
}

// edgeRequestClientImpl implements EdgeRequestClient
type edgeRequestClientImpl struct {
	ipc.RequestResponseDriver
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
		RequestResponseDriver: ipc.RequestResponseDriver{
			Component: goutils.Component{
				LogTags: logTags,
				LogTagModifiers: []goutils.LogMetadataModifier{
					goutils.ModifyLogMetadataByRestRequestParam,
				},
			},
			Client: coreClient,
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
		response = ipc.NewGetGeneralResponse(false, err.Error())
	} else {
		response = ipc.NewGetVideoSourceByNameResponse(sourceInfo)
	}

	return response, nil
}

// ======================================================================================
// Outbound Request Processing

func (c *edgeRequestClientImpl) ChangeVideoStreamingState(
	ctxt context.Context, sourceID string, newState bool,
) error {
	logTags := c.GetLogTagsForContext(ctxt)

	// BUild the request
	msg := ipc.NewChangeSourceStreamingStateRequest(sourceID, newState)
	_, err := json.Marshal(&msg)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Fail to build streaming state change IPC msg")
		return nil
	}

	// TODO: Continue

	return nil
}
