package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
)

// PlaylistForwardCB callback signature of function for sending newly received HLS playlist
// onward for processing
type PlaylistForwardCB func(context.Context, hls.Playlist, time.Time) error

// PlaylistReceiveHandler HLS playlist receiver
//
// This is only meant to be used by the edge node
type PlaylistReceiveHandler struct {
	goutils.RestAPIHandler
	parser          hls.PlaylistParser
	forwardPlaylist PlaylistForwardCB
}

/*
NewPlaylistReceiveHandler define a new HLS playlist receiver

	@param playlistParser hls.PlaylistParser - playlist parser object
	@param forwardPlaylist - callback function for sending newly received HLS playlist
		onward for processing
	@param logConfig common.HTTPRequestLogging - handler log settings
	@returns new PlaylistReceiveHandler
*/
func NewPlaylistReceiveHandler(
	playlistParser hls.PlaylistParser,
	forwardPlaylist PlaylistForwardCB,
	logConfig common.HTTPRequestLogging,
) (PlaylistReceiveHandler, error) {
	return PlaylistReceiveHandler{
		RestAPIHandler: goutils.RestAPIHandler{
			Component: goutils.Component{
				LogTags: log.Fields{"module": "api", "component": "edge-api-handler"},
				LogTagModifiers: []goutils.LogMetadataModifier{
					goutils.ModifyLogMetadataByRestRequestParam,
				},
			},
			CallRequestIDHeaderField: &logConfig.RequestIDHeader,
			DoNotLogHeaders: func() map[string]bool {
				result := map[string]bool{}
				for _, v := range logConfig.DoNotLogHeaders {
					result[v] = true
				}
				return result
			}(),
		}, parser: playlistParser, forwardPlaylist: forwardPlaylist,
	}, nil
}

// NewPlaylist godoc
// @Summary Process new HLS playlist
// @Description Receives new HLS playlist for processing. The sender must also provide the
// location where the MPEG-TS segment are stored as a URI prefix. The HLS playlist monitor
// will use that prefix to pull the segments into the system.
// @tags edge
// @Accept plain
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param MPEG-TS-URI-Prefix header string true "URI prefix for the MPEG-TS segments. The assumption is that all segments listed in the playlist are relative to this prefix."
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/playlist [post]
func (h PlaylistReceiveHandler) NewPlaylist(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get the MPEG-TS prefix
	tsFileURIPrefix := r.Header.Get("MPEG-TS-URI-Prefix")
	if tsFileURIPrefix == "" {
		msg := "request missing MPEG-TS-URI-Prefix"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Parse the body for the playlist
	content, err := io.ReadAll(r.Body)
	if err != nil {
		msg := "unable to read playlist from request"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
		return
	}
	defer func() {
		if err = r.Body.Close(); err != nil {
			log.WithError(err).WithFields(logTags).Error("Request body close error")
		}
	}()

	// Prepare the playlist for parsing
	playlistRaw := []string{}
	for _, oneLine := range strings.Split(string(content), "\n") {
		playlistRaw = append(playlistRaw, strings.TrimSpace(oneLine))
	}
	dummyPlaylistURI := fmt.Sprintf("%s/vid.m3u8", tsFileURIPrefix)

	currentTime := time.Now().UTC()

	// Parse the playlist
	playlist, err := h.parser.ParsePlaylist(r.Context(), dummyPlaylistURI, playlistRaw, currentTime)
	if err != nil {
		msg := "unable to parse sent playlist"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
		return
	}

	// Forward the playlist for further processing
	if err := h.forwardPlaylist(r.Context(), playlist, currentTime); err != nil {
		msg := "playlist processing failed"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	respCode = http.StatusOK
	response = h.GetStdRESTSuccessMsg(r.Context())
}

// NewPlaylistHandler Wrapper around NewPlaylist
func (h PlaylistReceiveHandler) NewPlaylistHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.NewPlaylist(w, r)
	}
}
