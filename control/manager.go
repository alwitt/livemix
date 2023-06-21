package control

import (
	"context"

	"github.com/alwitt/livemix/common"
)

// SystemManager system operations manager
type SystemManager interface {
	/*
		Ready check whether the manager is ready

			@param ctxt context.Context - execution context
	*/
	Ready(ctxt context.Context) error

	// =====================================================================================
	// Video sources

	/*
		DefineVideoSource create new video source

			@param ctxt context.Context - execution context
			@param name string - source name
			@param playlistURI string - video source playlist URI
			@param description *string - optionally, source description
			@returns new source entry ID
	*/
	DefineVideoSource(
		ctxt context.Context, name, playlistURI string, description *string,
	) (string, error)

	/*
		GetVideoSource retrieve a video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
			@returns video source entry
	*/
	GetVideoSource(ctxt context.Context, id string) (common.VideoSource, error)

	/*
		GetVideoSourceByName retrieve a video source by name

			@param ctxt context.Context - execution context
			@param name string - source name
			@returns video source entry
	*/
	GetVideoSourceByName(ctxt context.Context, name string) (common.VideoSource, error)

	/*
		ListVideoSources list all video sources

			@param ctxt context.Context - execution context
			@returns all video source entries
	*/
	ListVideoSources(ctxt context.Context) ([]common.VideoSource, error)

	/*
		UpdateVideoSource update properties of a video source.

		Only the following can be updated:
		  * Name
		  * Description
		  * Playlist URI

			@param ctxt context.Context - execution context
			@param newSetting common.VideoSource - new properties
	*/
	UpdateVideoSource(ctxt context.Context, newSetting common.VideoSource) error

	/*
		DeleteVideoSource delete a video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
	*/
	DeleteVideoSource(ctxt context.Context, id string) error
}
