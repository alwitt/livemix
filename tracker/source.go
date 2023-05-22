package tracker

import "github.com/alwitt/livemix/hls"

// SourceHLSTracker monitor status of a HLS video source
type SourceHLSTracker interface {
	/*
		Update update status of the HLS source based on information in the current playlist

			@param currentPlaylist hls.Playlist - the current playlist for the video source
	*/
	Update(currentPlaylist hls.Playlist) error
}
