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
	ipcRequestTypeGetVideoSource       baseRequestTypeDT = "get_video_source"
	ipcRequestTypeChangeStreamState    baseRequestTypeDT = "change_stream_state"
	ipcRequestTypeStartRecording       baseRequestTypeDT = "start_recording"
	ipcRequestTypeStopRecording        baseRequestTypeDT = "stop_recording"
	ipcRequestTypeCloseAllRecording    baseRequestTypeDT = "close_all_active_recording"
	ipcRequestTypeListActiveRecordings baseRequestTypeDT = "list_active_recordings"
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
	NewState int `json:"new_state" validate:"oneof=-1 1"`
}

/*
NewChangeSourceStreamingStateRequest define new ChangeSourceStreamingStateRequest message

	@param sourceID string - video source ID
	@param newState int - new streaming state
	@returns defined structure
*/
func NewChangeSourceStreamingStateRequest(
	sourceID string, newState int,
) ChangeSourceStreamingStateRequest {
	return ChangeSourceStreamingStateRequest{
		BaseRequest: BaseRequest{
			BaseMessage: BaseMessage{Type: ipcMessageTypeRequest},
			RequestType: ipcRequestTypeChangeStreamState,
		}, SourceID: sourceID, NewState: newState,
	}
}

// StartVideoRecordingRequest RR request start video recording session
type StartVideoRecordingRequest struct {
	BaseRequest
	// Session information regarding the recording session
	Session common.Recording `json:"session" validate:"required,dive"`
}

/*
NewStartVideoRecordingSessionRequest define new StartVideoRecordingSessionRequest message

	@param session common.Recording - the video recording session
	@returns defined structure
*/
func NewStartVideoRecordingSessionRequest(session common.Recording) StartVideoRecordingRequest {
	return StartVideoRecordingRequest{
		BaseRequest: BaseRequest{
			BaseMessage: BaseMessage{Type: ipcMessageTypeRequest},
			RequestType: ipcRequestTypeStartRecording,
		}, Session: session,
	}
}

// StopVideoRecordingRequest RR request stop video recording session
type StopVideoRecordingRequest struct {
	BaseRequest
	// RecordingID recording session ID
	RecordingID string `json:"session_id" validate:"required"`
	// EndTime the end point for the recording session
	EndTime time.Time `json:"end_time" validate:"required"`
}

/*
NewStopVideoRecordingSessionRequest define new StopVideoRecordingSessionRequest message

	@param recordingID string - recording session ID
	@param endTime time.Time - recording session end point
	@returns defined structure
*/
func NewStopVideoRecordingSessionRequest(
	recordingID string, endTime time.Time,
) StopVideoRecordingRequest {
	return StopVideoRecordingRequest{
		BaseRequest: BaseRequest{
			BaseMessage: BaseMessage{Type: ipcMessageTypeRequest},
			RequestType: ipcRequestTypeStopRecording,
		}, RecordingID: recordingID, EndTime: endTime,
	}
}

// CloseAllActiveRecordingRequest RR request close all active recording sessions
type CloseAllActiveRecordingRequest struct {
	BaseRequest
	// SourceID video source ID
	SourceID string `json:"source_id" validate:"required"`
}

/*
NewCloseAllActiveRecordingRequest define new CloseAllActiveRecordingRequest message

	@param sourceID string - video source ID
	@returns defined structure
*/
func NewCloseAllActiveRecordingRequest(sourceID string) CloseAllActiveRecordingRequest {
	return CloseAllActiveRecordingRequest{
		BaseRequest: BaseRequest{
			BaseMessage: BaseMessage{Type: ipcMessageTypeRequest},
			RequestType: ipcRequestTypeCloseAllRecording,
		}, SourceID: sourceID,
	}
}

// ListActiveRecordingsRequest RR request list all active recording sessions associated
// with a video source
type ListActiveRecordingsRequest struct {
	BaseRequest
	// SourceID video source ID
	SourceID string `json:"source_id" validate:"required"`
}

/*
NewListActiveRecordingsRequest define new ListActiveRecordingsRequest message

	@param sourceID string - video source ID
	@returns defined structure
*/
func NewListActiveRecordingsRequest(sourceID string) ListActiveRecordingsRequest {
	return ListActiveRecordingsRequest{
		BaseRequest: BaseRequest{
			BaseMessage: BaseMessage{Type: ipcMessageTypeRequest},
			RequestType: ipcRequestTypeListActiveRecordings,
		}, SourceID: sourceID,
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

	// Start Video Recording
	case ipcRequestTypeStartRecording:
		var request StartVideoRecordingRequest
		if err := json.Unmarshal(rawMsg, &request); err != nil {
			return nil, err
		}
		return request, nil

	// Stop Video Recording
	case ipcRequestTypeStopRecording:
		var request StopVideoRecordingRequest
		if err := json.Unmarshal(rawMsg, &request); err != nil {
			return nil, err
		}
		return request, nil

	// Close All Active Recording Sessions
	case ipcRequestTypeCloseAllRecording:
		var request CloseAllActiveRecordingRequest
		if err := json.Unmarshal(rawMsg, &request); err != nil {
			return nil, err
		}
		return request, nil

	// List All Active Recording Sessions Associated With Video Source
	case ipcRequestTypeListActiveRecordings:
		var request ListActiveRecordingsRequest
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
	ipcResponseTypeGeneralResponse  baseResponseTypeDT = "general_resp"
	ipcResponseTypeGetVideoSource   baseResponseTypeDT = "get_video_source_resp"
	ipcResponseTypeActiveRecordings baseResponseTypeDT = "list_active_recordings_resp"
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
NewGeneralResponse define new GeneralResponse message

	@param success bool - whether the request succeeded
	@param errorMsg string - if request failed, the error message
	@returns defined structure
*/
func NewGeneralResponse(success bool, errorMsg string) GeneralResponse {
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

// ListActiveRecordingsResponse RR response containing a list of active recording sessions
type ListActiveRecordingsResponse struct {
	BaseResponse
	// Recordings list of active recordings
	Recordings []common.Recording `json:"recordings" validate:"required,gte=1,dive"`
}

/*
NewListActiveRecordingsResponse define new ListActiveRecordingsResponse message

	@param recordings []common.Recording - list of active recording sessions
	@returns defined structure
*/
func NewListActiveRecordingsResponse(recordings []common.Recording) ListActiveRecordingsResponse {
	return ListActiveRecordingsResponse{
		BaseResponse: BaseResponse{
			BaseMessage:  BaseMessage{Type: ipcMessageTypeResponse},
			ResponseType: ipcResponseTypeActiveRecordings,
		},
		Recordings: recordings,
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

	// List Of Active Recording Sessions Response
	case ipcResponseTypeActiveRecordings:
		var response ListActiveRecordingsResponse
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
	ipcBroadcastVideoSourceStatus   baseBroadcastTypeDT = "report_video_source_status"
	ipcBroadcastNewRecordingSegment baseBroadcastTypeDT = "new_video_recording_segment"
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

// RecordingSegmentReport broadcast new video segments associated with recording sessions
type RecordingSegmentReport struct {
	BaseBroadcast
	// RecordingIDs set of recording sessions ID which the segments belong to
	RecordingIDs []string `json:"recordings" validate:"required,gte=1"`
	// Segments set of video segments to report
	Segments []common.VideoSegment `json:"segments" validate:"required,gte=1,dive"`
}

/*
NewRecordingSegmentReport define new RecordingSegmentReport message

	@param recordingIDs []string - set of recording sessions ID which the segments belong to
	@param segments []common.VideoSegment - set of video segments to report
	@returns defined structure
*/
func NewRecordingSegmentReport(
	recordingIDs []string, segments []common.VideoSegment,
) RecordingSegmentReport {
	return RecordingSegmentReport{
		BaseBroadcast: BaseBroadcast{
			BaseMessage:   BaseMessage{Type: ipcMessageTypeBroadcast},
			BroadcastType: ipcBroadcastNewRecordingSegment,
		}, RecordingIDs: recordingIDs, Segments: segments,
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

	// New Recording Video Segments
	case ipcBroadcastNewRecordingSegment:
		var broadcast RecordingSegmentReport
		if err := json.Unmarshal(rawMsg, &broadcast); err != nil {
			return nil, err
		}
		return broadcast, nil

	default:
		return nil, fmt.Errorf("unknown IPC broadcast message type '%s'", asBroadcastMsg.Type)
	}
}
