package ipc

import (
	"encoding/json"
	"fmt"

	"github.com/alwitt/livemix/common"
)

type baseMessageTypeDT string

const (
	ipcMessageTypeRequest  baseMessageTypeDT = "request"
	ipcMessageTypeResponse baseMessageTypeDT = "response"
)

// BaseMessage base request-response IPC message payload. All other messages must be built
// upon this structure.
type BaseMessage struct {
	// Type the message type
	Type baseMessageTypeDT `json:"type" validate:"required,oneof=request response"`
}

/*
ParseRawMessage parse raw IPC message

	@param rawMsg []byte - original message
	@returns parsed message
*/
func ParseRawMessage(rawMsg []byte) (interface{}, error) {
	var asBaseMsg BaseMessage
	if err := json.Unmarshal(rawMsg, &asBaseMsg); err != nil {
		return nil, err
	}
	// Based on the type, decide how to parse
	switch asBaseMsg.Type {
	case ipcMessageTypeRequest:
		return parseRawRequestMessage(rawMsg)
	case ipcMessageTypeResponse:
		return parseRawResponseMessage(rawMsg)
	default:
		return nil, fmt.Errorf("unknown IPC message type '%s'", asBaseMsg.Type)
	}
}

// =====================================================================================
// IPC Request Messages

type baseRequestTypeDT string

const (
	ipcRequestTypeGetVideoSource baseRequestTypeDT = "get_video_source"
)

// BaseRequest base request IPC message payload. All other requests must be built
// upon this structure.
type BaseRequest struct {
	BaseMessage
	// RequestType the request type
	RequestType baseRequestTypeDT `json:"request_type" validate:"required,oneof=get_video_source"`
}

// GetVideoSourceByNameRequest RR request info for video source by name
type GetVideoSourceByNameRequest struct {
	BaseRequest
	// TargetName the video source name to query for
	TargetName string `json:"target_name" validate:"required"`
}

/*
NewGetVideoSourceByNameRequest define new GetVideoSourceByNameRequest message

	@param sourceName string - video source name
	@returns defined structure
*/
func NewGetVideoSourceByNameRequest(sourceName string) GetVideoSourceByNameRequest {
	return GetVideoSourceByNameRequest{
		BaseRequest: BaseRequest{
			BaseMessage: BaseMessage{Type: ipcMessageTypeRequest},
			RequestType: ipcRequestTypeGetVideoSource,
		},
		TargetName: sourceName,
	}
}

/*
parseRawRequestMessage parse raw IPC request message

	@param rawMsg []byte - original message
	@returns parsed message
*/
func parseRawRequestMessage(rawMsg []byte) (interface{}, error) {
	var asRequestMsg BaseRequest
	if err := json.Unmarshal(rawMsg, &asRequestMsg); err != nil {
		return nil, err
	}
	// Based on the type, parse
	switch asRequestMsg.RequestType {
	// Request Video Source Info
	case ipcRequestTypeGetVideoSource:
		var getVideoSourceReq GetVideoSourceByNameRequest
		if err := json.Unmarshal(rawMsg, &getVideoSourceReq); err != nil {
			return nil, err
		}
		return getVideoSourceReq, nil

	default:
		return nil, fmt.Errorf("unknown IPC request message type '%s'", asRequestMsg.Type)
	}
}

// =====================================================================================
// IPC Response Messages

type baseResponseTypeDT string

const (
	ipcResponseTypeGetVideoSource baseResponseTypeDT = "get_video_source_resp"
)

// BaseResponse base response IPC message payload. All other responses must be built
// upon this structure.
type BaseResponse struct {
	BaseMessage
	ResponseType baseResponseTypeDT `json:"response_type" validate:"required,oneof=get_video_source_resp"`
}

// GetVideoSourceByNameResponse RR response containing video source info
type GetVideoSourceByNameResponse struct {
	BaseResponse
	// Source the video source info
	Source common.VideoSource `json:"source" validate:"required,dive"`
}

/*
NewGetVideoSourceByNameResponse define new GetVideoSourceByNameResponse message

	@param sourceInfo common.VideoSource - video source info
	@returns defined structure
*/
func NewGetVideoSourceByNameResponse(sourceInfo common.VideoSource) GetVideoSourceByNameResponse {
	return GetVideoSourceByNameResponse{
		BaseResponse: BaseResponse{
			BaseMessage:  BaseMessage{Type: ipcMessageTypeResponse},
			ResponseType: ipcResponseTypeGetVideoSource,
		},
		Source: sourceInfo,
	}
}

/*
parseRawResponseMessage parse raw IPC response message

	@param rawMsg []byte - original message
	@returns parsed message
*/
func parseRawResponseMessage(rawMsg []byte) (interface{}, error) {
	var asResponseMsg BaseResponse
	if err := json.Unmarshal(rawMsg, &asResponseMsg); err != nil {
		return nil, err
	}
	// Based on the type, parse
	switch asResponseMsg.ResponseType {
	// Video Source Info Response
	case ipcResponseTypeGetVideoSource:
		var getVideoSourceResp GetVideoSourceByNameResponse
		if err := json.Unmarshal(rawMsg, &getVideoSourceResp); err != nil {
			return nil, err
		}
		return getVideoSourceResp, nil

	default:
		return nil, fmt.Errorf("unknown IPC response message type '%s'", asResponseMsg.Type)
	}
}
