package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/alwitt/goutils"
	"github.com/apex/log"
)

// InboundRequestHandler callback function to process a specific inbound request
type InboundRequestHandler func(
	ctxt context.Context, request interface{}, origMsg goutils.ReqRespMessage,
) (interface{}, error)

// RequestResponseDriver wrapper class to combine common code needed by request-response users
type RequestResponseDriver struct {
	goutils.Component
	Client       goutils.RequestResponseClient
	executionMap map[reflect.Type]InboundRequestHandler
}

/*
InstallHandler install a handler for an inbound request

	@param requestType reflect.Type - request message type
	@param handler InboundRequestHandler - request handler callback
*/
func (d *RequestResponseDriver) InstallHandler(
	requestType reflect.Type, handler InboundRequestHandler,
) {
	if d.executionMap == nil {
		d.executionMap = map[reflect.Type]InboundRequestHandler{}
	}
	d.executionMap[requestType] = handler
}

/*
ProcessInboundRequest process inbound request

	@param ctxt context.Context - execution context
	@param msg goutils.ReqRespMessage - raw request message
*/
func (d *RequestResponseDriver) ProcessInboundRequest(
	ctxt context.Context, msg goutils.ReqRespMessage,
) error {
	logTag := d.GetLogTagsForContext(ctxt)

	// Parse the message to determine the request
	parsed, err := ParseRawMessage(msg.Payload)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", msg.SenderID).
			WithField("request-id", msg.RequestID).
			Error("Unable to parse request payload")
		return err
	}

	// Find the associated processing handler
	requestMsgType := reflect.TypeOf(parsed)
	requestHandler, ok := d.executionMap[requestMsgType]
	if !ok {
		err := fmt.Errorf("unknown supported request type '%s'", requestMsgType)
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", msg.SenderID).
			WithField("request-id", msg.RequestID).
			WithField("request-type", requestMsgType).
			Error("Unable to parse request payload")
		return err
	}

	// Process request
	response, err := requestHandler(ctxt, parsed, msg)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", msg.SenderID).
			WithField("request-id", msg.RequestID).
			WithField("request-type", requestMsgType).
			Error("Request processing failed")
		return err
	}

	// Build the response
	respMsg, err := json.Marshal(&response)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", msg.SenderID).
			WithField("request-id", msg.RequestID).
			WithField("request-type", requestMsgType).
			Error("Failed to prepare response")
		return err
	}

	// Send the response
	if err := d.Client.Respond(ctxt, msg, respMsg, nil, false); err != nil {
		log.
			WithError(err).
			WithFields(logTag).
			WithField("request-sender", msg.SenderID).
			WithField("request-id", msg.RequestID).
			WithField("request-type", requestMsgType).
			Error("Failed to send response")
		return err
	}

	return nil
}

/*
MakeRequest wrapper function to marking an outbound request

	@param ctxt context.Context - execution context
	@param requestInstanceName string - descriptive name for this request to identify it in logs
	@param targetID string - request target ID
	@param requestMsg []byte - request payload
	@param requestMeta map[string]string - request's associated metadata
	@param callParam goutils.RequestCallParam - request call parameters
	@returns response payload or payloads if multiple responses expected
*/
func (d *RequestResponseDriver) MakeRequest(
	ctxt context.Context,
	requestInstanceName string,
	targetID string,
	requestMsg []byte,
	requestMeta map[string]string,
	callParam goutils.RequestCallParam,
) ([]interface{}, error) {
	logTags := d.GetLogTagsForContext(ctxt)

	// Handler to receive the message from control
	respReceiveChan := make(chan goutils.ReqRespMessage, callParam.ExpectedResponsesCount+1)
	respReceiveCB := func(ctxt context.Context, msg goutils.ReqRespMessage) error {
		log.
			WithFields(logTags).
			WithField("request-instance", requestInstanceName).
			Debug("Received response")
		respReceiveChan <- msg
		return nil
	}
	// Handler in case of request timeout
	timeoutChan := make(chan error, 2)
	timeoutCB := func(ctxt context.Context) error {
		err := fmt.Errorf(requestInstanceName)
		log.
			WithError(err).
			WithFields(logTags).
			WithField("request-instance", requestInstanceName).
			Debug("No responses received before timeout")
		timeoutChan <- err
		return nil
	}

	// Update call parameter locally defined callbacks
	callParam.RespHandler = respReceiveCB
	callParam.TimeoutHandler = timeoutCB

	var rawResponse goutils.ReqRespMessage

	log.
		WithFields(logTags).
		WithField("request-instance", requestInstanceName).
		Debug("Sending request")
	// Make the call
	requestID, err := d.Client.Request(ctxt, targetID, requestMsg, requestMeta, callParam)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("request-id", requestID).
			WithField("request-instance", requestInstanceName).
			Error("Failed to send request")
		return nil, err
	}
	log.
		WithFields(logTags).
		WithField("request-id", requestID).
		WithField("request-instance", requestInstanceName).
		Debug("Sent request. Waiting for response...")

	results := []interface{}{}
	// Wait for responses from target/s
	for itr := 0; itr < callParam.ExpectedResponsesCount; itr++ {
		select {
		// Execution context timeout
		case <-ctxt.Done():
			return nil, fmt.Errorf("execution context timed out")

		// Request timeout
		case <-timeoutChan:
			err = fmt.Errorf("timeout channel returned erroneous results")
			log.
				WithError(err).
				WithFields(logTags).
				WithField("request-id", requestID).
				WithField("request-instance", requestInstanceName).
				Error("Request failed")
			return nil, err

		// Response successful
		case resp, ok := <-respReceiveChan:
			if !ok {
				err := fmt.Errorf("response channel returned erroneous results")
				log.
					WithError(err).
					WithFields(logTags).
					WithField("request-id", requestID).
					WithField("request-instance", requestInstanceName).
					Error("Request failed")
				return nil, err
			}
			rawResponse = resp
		}

		log.
			WithFields(logTags).
			WithField("request-id", requestID).
			WithField("request-instance", requestInstanceName).
			WithField("raw-msg", string(rawResponse.Payload)).
			Debug("Raw response payload")

		// Parse the response
		parsed, err := ParseRawMessage(rawResponse.Payload)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("request-id", requestID).
				WithField("request-instance", requestInstanceName).
				Error("Unable to parse response")
			return nil, err
		}
		results = append(results, parsed)
	}

	return results, nil
}
