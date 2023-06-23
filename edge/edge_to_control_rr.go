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

// ControlRequestClient request-response client for edge to call control
type ControlRequestClient interface {
	/*
		GetVideoSourceInfo query control for a video source's information

			@param ctxt context.Context - execution context
			@param sourceName string - video source name
			@returns video source info
	*/
	GetVideoSourceInfo(ctxt context.Context, sourceName string) (common.VideoSource, error)
}

// controlRequestClientImpl implements ControlRequestClient
type controlRequestClientImpl struct {
	goutils.Component
	controlRRTargetID string
	client            goutils.RequestResponseClient
	requestTimeout    time.Duration
	validator         *validator.Validate
}

/*
NewControlRequestClient define a new edge to control request-response client

	@param clientName string - name of this client instance
	@param controlRRTargetID string - control's target ID for request-response targeting
	@param coreClient goutils.RequestResponseClient - core request-response client
	@param requestTimeout time.Duration - request-response request timeout
	@returns new client
*/
func NewControlRequestClient(
	clientName string,
	controlRRTargetID string,
	coreClient goutils.RequestResponseClient,
	requestTimeout time.Duration,
) (ControlRequestClient, error) {
	logTags := log.Fields{
		"module": "api", "component": "edge-to-control-rr-client", "instance": clientName,
	}
	return &controlRequestClientImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		controlRRTargetID: controlRRTargetID,
		client:            coreClient,
		requestTimeout:    requestTimeout,
		validator:         validator.New(),
	}, nil
}

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

	// Handler to receive the message from control
	respReceiveChan := make(chan goutils.ReqRespMessage, 2)
	respReceiveCB := func(ctxt context.Context, msg goutils.ReqRespMessage) error {
		log.
			WithFields(logTags).
			Debug("Received raw video source info response")
		respReceiveChan <- msg
		return nil
	}
	// Handler in case of request timeout
	timeoutChan := make(chan error, 2)
	timeoutCB := func(ctxt context.Context) error {
		err := fmt.Errorf("video source '%s' info request timeout", sourceName)
		log.
			WithError(err).
			WithFields(logTags).
			Debug("Video source info query failed")
		timeoutChan <- err
		return nil
	}

	// Prepare the request parameters
	callParam := goutils.RequestCallParam{
		RespHandler:            respReceiveCB,
		ExpectedResponsesCount: 1,
		Blocking:               false,
		Timeout:                c.requestTimeout,
		TimeoutHandler:         timeoutCB,
	}

	var rawResponse goutils.ReqRespMessage

	log.
		WithFields(logTags).
		Debugf("Sending video source '%s' info request", sourceName)
	// Make the call
	requestID, err := c.client.Request(ctxt, c.controlRRTargetID, msgStr, nil, callParam)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("request-id", requestID).
			Errorf("Failed to send video source '%s' info request", sourceName)
		return common.VideoSource{}, err
	}
	log.
		WithFields(logTags).
		WithField("request-id", requestID).
		Debugf("Sent video source '%s' info request. Waiting for response...", sourceName)

	// Wait for response from control
	select {
	// Execution context timeout
	case <-ctxt.Done():
		return common.VideoSource{}, fmt.Errorf("execution context timed out")

	// Request timeout
	case err, ok := <-timeoutChan:
		if ok {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("request-id", requestID).
				Errorf("Video source '%s' info request failed", sourceName)
			return common.VideoSource{}, err
		}
		err = fmt.Errorf("request timeout channel returned erroneous results")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("request-id", requestID).
			Errorf("Video source '%s' info request failed", sourceName)
		return common.VideoSource{}, err

	// Response successful
	case resp, ok := <-respReceiveChan:
		if !ok {
			err := fmt.Errorf("response channel returned erroneous results")
			log.
				WithError(err).
				WithFields(logTags).
				WithField("request-id", requestID).
				Errorf("Video source '%s' info request failed", sourceName)
			return common.VideoSource{}, err
		}
		rawResponse = resp
	}

	log.
		WithFields(logTags).
		WithField("raw-msg", string(rawResponse.Payload)).
		Debugf("Controller returned video source '%s' info", sourceName)

	// Process the response
	parsed, err := ipc.ParseRawMessage(rawResponse.Payload)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Errorf("Unable to parse video source '%s' info", sourceName)
		return common.VideoSource{}, err
	}
	videoInfo, ok := parsed.(ipc.GetVideoSourceByNameResponse)
	if !ok {
		err := fmt.Errorf("received unexpected response message (%s)", reflect.TypeOf(parsed))
		log.
			WithError(err).
			WithFields(logTags).
			Errorf("Unable to parse video source '%s' info", sourceName)
		return common.VideoSource{}, err
	}
	if err := c.validator.Struct(&videoInfo); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Can't process invalid 'GetVideoSourceByNameResponse' from control")
		return common.VideoSource{}, err
	}

	return videoInfo.Source, nil
}
