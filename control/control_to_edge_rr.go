package control

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/apex/log"
)

// EdgeRequestClient request-response client for control to call edge
type EdgeRequestClient interface {
	/*
		InstallReferenceToManager used by a SystemManager to add a reference of itself into the client

			@param newManager SystemManager - reference to the manager
	*/
	InstallReferenceToManager(newManager SystemManager)
}

// edgeRequestClientImpl implements EdgeRequestClient
type edgeRequestClientImpl struct {
	goutils.Component
	client goutils.RequestResponseClient
	core   SystemManager
}

/*
NewEdgeRequestClient define a new control to edge request-response client

	@param clientName string - name of this client instance
	@param coreClient goutils.RequestResponseClient - core request-response client
	@returns new client
*/
func NewEdgeRequestClient(
	ctxt context.Context,
	clientName string,
	coreClient goutils.RequestResponseClient,
) (EdgeRequestClient, error) {
	logTags := log.Fields{
		"module": "api", "component": "control-to-edge-rr-client", "instance": clientName,
	}

	instance := &edgeRequestClientImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		client: coreClient,
		core:   nil,
	}

	// Install inbound request handling
	if err := coreClient.SetInboundRequestHandler(ctxt, instance.processInboundRequest); err != nil {
		return nil, err
	}

	return instance, nil
}

func (c *edgeRequestClientImpl) InstallReferenceToManager(newManager SystemManager) {
	c.core = newManager
}

func (c *edgeRequestClientImpl) processInboundRequest(
	ctxt context.Context, msg goutils.ReqRespMessage,
) error {
	logTag := c.GetLogTagsForContext(ctxt)

	// Parse the message to determine the request
	parsed, err := ipc.ParseRawMessage(msg.Payload)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", msg.SenderID).
			WithField("request-id", msg.RequestID).
			Error("Unable to parse request payload")
		return err
	}

	if c.core == nil {
		err := fmt.Errorf("no reference to SystemManager set yet")
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", msg.SenderID).
			WithField("request-id", msg.RequestID).
			Error("Unable to start request handling")
		return err
	}

	switch reflect.TypeOf(parsed) {
	case reflect.TypeOf(ipc.GetVideoSourceByNameRequest{}):
		// TODO FIXME: add validation check
		request := parsed.(ipc.GetVideoSourceByNameRequest)
		// Process video source info request
		sourceInfo, err := c.core.GetVideoSourceByName(ctxt, request.TargetName)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTag).
				WithField("request-sender", msg.SenderID).
				WithField("request-id", msg.RequestID).
				Errorf("Read video source '%s' info", request.TargetName)
			return err
		}
		// Built the response
		response := ipc.NewGetVideoSourceByNameResponse(sourceInfo)
		respMsg, err := json.Marshal(&response)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTag).
				WithField("request-sender", msg.SenderID).
				WithField("request-id", msg.RequestID).
				Errorf("Failed to prepare video source '%s' info response", request.TargetName)
			return err
		}
		// Send the response
		if err := c.client.Respond(ctxt, msg, respMsg, nil, false); err != nil {
			log.
				WithError(err).
				WithFields(logTag).
				WithField("request-sender", msg.SenderID).
				WithField("request-id", msg.RequestID).
				Errorf("Failed to send video source '%s' info response", request.TargetName)
			return err
		}

	default:
		err := fmt.Errorf("unknown supported request type '%s'", reflect.TypeOf(parsed))
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", msg.SenderID).
			WithField("request-id", msg.RequestID).
			Error("Unable to parse request payload")
		return err
	}

	return nil
}
