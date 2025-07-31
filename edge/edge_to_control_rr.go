package edge

import (
	"context"
	"time"

	"github.com/alwitt/livemix/common"
)

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
		ListActiveRecordingsOfSource list all active video recording sessions of a video source

			@param ctxt context.Context - execution context
			@param sourceID string - the video source ID
			@returns all active recording sessions of a video source
	*/
	ListActiveRecordingsOfSource(ctxt context.Context, sourceID string) ([]common.Recording, error)

	/*
		ExchangeVideoSourceStatus exchange video source status report with control

			@param ctxt context.Context - execution context
			@param sourceID string - the video source ID
			@param reqRespTargetID string - the request-response target ID for reaching video source
			    over request-response network.
			@param localTime time.Time - video source local time
			@returns: current state of the video source from control side, and any active
			    recordings associated with the video source.
	*/
	ExchangeVideoSourceStatus(
		ctxt context.Context, sourceID string, reqRespTargetID string, localTime time.Time,
	) (common.VideoSource, []common.Recording, error)

	/*
		StopAllAssociatedRecordings request all recording associated this this source is stopped

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
	*/
	StopAllAssociatedRecordings(ctxt context.Context, sourceID string) error
}
