package edge

import "context"

// VideoSourceOperator video source operations manager
type VideoSourceOperator interface {
	/*
		Ready check whether the DB connection is working

			@param ctxt context.Context - execution context
	*/
	Ready(ctxt context.Context) error

	// =====================================================================================
	// Video sources

	/*
		RecordKnownVideoSource create record for a known video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
			@param name string - source name
			@param playlistURI *string - video source playlist URI
			@param description *string - optionally, source description
			@param streaming bool - whether the video source is currently streaming
	*/
	RecordKnownVideoSource(
		ctxt context.Context, id, name string, playlistURI, description *string, streaming bool,
	) error

	/*
		ChangeVideoSourceStreamState change the streaming state for a video source

			@param ctxt context.Context - execution context
			@param id string - source ID
			@param streaming bool - new streaming state
	*/
	ChangeVideoSourceStreamState(ctxt context.Context, id string, streaming bool) error
}
