package ipc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/alwitt/livemix/common"
)

type baseMessageTypeDT string

const (
	ipcMessageTypeRequest   baseMessageTypeDT = "request"
	ipcMessageTypeResponse  baseMessageTypeDT = "response"
	ipcMessageTypeBroadcast baseMessageTypeDT = "broadcast"
)

// BaseMessage base request-response IPC message payload. All other messages must be built
// upon this structure.
type BaseMessage struct {
	// Type the message type
	Type baseMessageTypeDT `json:"type" validate:"required,oneof=request response broadcast"`
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
	case ipcMessageTypeBroadcast:
		return parseRawBroadcastMessage(rawMsg)
	default:
		return nil, fmt.Errorf("unknown IPC message type '%s'", asBaseMsg.Type)
	}
}

// =====================================================================================
// IPC Request Messages

type baseRequestTypeDT string

const (
	ipcRequestTypeGetVideoSource    baseRequestTypeDT = "get_video_source"
	ipcRequestTypeChangeStreamState baseRequestTypeDT = "change_stream_state"
)

// BaseRequest base request IPC message payload. All other requests must be built
// upon this structure.
type BaseRequest struct {
	BaseMessage
	// RequestType the request type
	RequestType baseRequestTypeDT `json:"request_type" validate:"required,oneof=get_video_source change_stream_state"`
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

// ChangeSourceStreamingStateRequest RR request change video source streaming type
type ChangeSourceStreamingStateRequest struct {
	BaseRequest
	// SourceID video source ID
	SourceID string `json:"source_id" validate:"required"`
	// NewState new video source streaming state
	NewState bool `json:"new_state"`
}

/*
NewChangeSourceStreamingStateRequest define new ChangeSourceStreamingStateRequest message

	@param sourceID string - video source ID
	@param newState bool - new streaming state
	@returns defined structure
*/
func NewChangeSourceStreamingStateRequest(
	sourceID string, newState bool,
) ChangeSourceStreamingStateRequest {
	return ChangeSourceStreamingStateRequest{
		BaseRequest: BaseRequest{
			BaseMessage: BaseMessage{Type: ipcMessageTypeRequest},
			RequestType: ipcRequestTypeChangeStreamState,
		}, SourceID: sourceID, NewState: newState,
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
		var request GetVideoSourceByNameRequest
		if err := json.Unmarshal(rawMsg, &request); err != nil {
			return nil, err
		}
		return request, nil

	// Request Change Streaming State
	case ipcRequestTypeChangeStreamState:
		var request ChangeSourceStreamingStateRequest
		if err := json.Unmarshal(rawMsg, &request); err != nil {
			return nil, err
		}
		return request, nil

	default:
		return nil, fmt.Errorf("unknown IPC request message type '%s'", asRequestMsg.Type)
	}
}

// =====================================================================================
// IPC Response Messages

type baseResponseTypeDT string

const (
	ipcResponseTypeGeneralResponse baseResponseTypeDT = "general_resp"
	ipcResponseTypeGetVideoSource  baseResponseTypeDT = "get_video_source_resp"
)

// BaseResponse base response IPC message payload. All other responses must be built
// upon this structure.
type BaseResponse struct {
	BaseMessage
	ResponseType baseResponseTypeDT `json:"response_type" validate:"required,oneof=general_resp get_video_source_resp"`
}

// GeneralResponse RR general purpose response
type GeneralResponse struct {
	BaseResponse
	// Success whether the request succeeded
	Success bool `json:"success"`
	// ErrorMsg if request failed, the error message
	ErrorMsg string `json:"error,omitempty"`
}

/*
NewGetGeneralResponse define new GeneralResponse message

	@param success bool - whether the request succeeded
	@param errorMsg string - if request failed, the error message
	@returns defined structure
*/
func NewGetGeneralResponse(success bool, errorMsg string) GeneralResponse {
	return GeneralResponse{
		BaseResponse: BaseResponse{
			BaseMessage:  BaseMessage{Type: ipcMessageTypeResponse},
			ResponseType: ipcResponseTypeGeneralResponse,
		}, Success: success, ErrorMsg: errorMsg,
	}
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
	// General Purpose Response
	case ipcResponseTypeGeneralResponse:
		var response GeneralResponse
		if err := json.Unmarshal(rawMsg, &response); err != nil {
			return nil, err
		}
		return response, nil

	// Video Source Info Response
	case ipcResponseTypeGetVideoSource:
		var response GetVideoSourceByNameResponse
		if err := json.Unmarshal(rawMsg, &response); err != nil {
			return nil, err
		}
		return response, nil

	default:
		return nil, fmt.Errorf("unknown IPC response message type '%s'", asResponseMsg.Type)
	}
}

// =====================================================================================
// IPC Broadcast Messages

type baseBroadcastTypeDT string

const (
	ipcBroadcastVideoSourceStatus baseBroadcastTypeDT = "report_video_source_status"
)

// BaseBroadcast base broadcast IPC message payload. All other broadcasts must be built
// upon this structure
type BaseBroadcast struct {
	BaseMessage
	// BroadcastType the broadcast type
	BroadcastType baseBroadcastTypeDT `json:"broadcast_type" validate:"required,oneof=report_video_source_status"`
}

// VideoSourceStatusReport broadcast video source status report
type VideoSourceStatusReport struct {
	BaseBroadcast
	// SourceID video source ID
	SourceID string `json:"source_id" validate:"required"`
	// RequestResponseTargetID the request-response target ID for reaching video source
	// over request-response network.
	RequestResponseTargetID string `json:"rr_target_id" validate:"required"`
	// LocalTimestamp video source local time
	LocalTimestamp time.Time `json:"local_timestamp"`
}

/*
NewVideoSourceStatusReport define new VideoSourceStatusReport message

	@param sourceID string - video source ID
	@param reqRespTargetID string - the request-response target ID for reaching video source
	    over request-response network.
	@param lclTimestamp time.Time - video source local time
	@returns defined structure
*/
func NewVideoSourceStatusReport(
	sourceID, reqRespTargetID string, lclTimestamp time.Time,
) VideoSourceStatusReport {
	return VideoSourceStatusReport{
		BaseBroadcast: BaseBroadcast{
			BaseMessage:   BaseMessage{Type: ipcMessageTypeBroadcast},
			BroadcastType: ipcBroadcastVideoSourceStatus,
		},
		SourceID:                sourceID,
		RequestResponseTargetID: reqRespTargetID,
		LocalTimestamp:          lclTimestamp,
	}
}

/*
parseRawBroadcastMessage parse raw IPC broadcast message

	@param rawMsg []byte - original message
	@returns parsed message
*/
func parseRawBroadcastMessage(rawMsg []byte) (interface{}, error) {
	var asBroadcastMsg BaseBroadcast
	if err := json.Unmarshal(rawMsg, &asBroadcastMsg); err != nil {
		return nil, err
	}
	// Based on the type, parse
	switch asBroadcastMsg.BroadcastType {
	// Video Source Status Report
	case ipcBroadcastVideoSourceStatus:
		var broadcast VideoSourceStatusReport
		if err := json.Unmarshal(rawMsg, &broadcast); err != nil {
			return nil, err
		}
		return broadcast, nil

	default:
		return nil, fmt.Errorf("unknown IPC broadcast message type '%s'", asBroadcastMsg.Type)
	}
}
