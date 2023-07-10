package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
)

// =====================================================================================
// Playlist Receiver - Hosted by edge nodes

// PlaylistForwardCB callback signature of function for sending newly received HLS playlist
// onward for processing
type PlaylistForwardCB func(context.Context, hls.Playlist, time.Time) error

// PlaylistReceiveHandler HLS playlist receiver
//
// This is only meant to be used by the edge node
type PlaylistReceiveHandler struct {
	goutils.RestAPIHandler
	parentCtxt      context.Context
	parser          hls.PlaylistParser
	forwardPlaylist PlaylistForwardCB
}

/*
NewPlaylistReceiveHandler define a new HLS playlist receiver

	@param parentCtxt context.Context - REST handler parent context
	@param playlistParser hls.PlaylistParser - playlist parser object
	@param forwardPlaylist - callback function for sending newly received HLS playlist
		onward for processing
	@param logConfig common.HTTPRequestLogging - handler log settings
	@returns new PlaylistReceiveHandler
*/
func NewPlaylistReceiveHandler(
	parentCtxt context.Context,
	playlistParser hls.PlaylistParser,
	forwardPlaylist PlaylistForwardCB,
	logConfig common.HTTPRequestLogging,
) (PlaylistReceiveHandler, error) {
	return PlaylistReceiveHandler{
		RestAPIHandler: goutils.RestAPIHandler{
			Component: goutils.Component{
				LogTags: log.Fields{"module": "api", "component": "playlist-receiver-handler"},
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
		}, parentCtxt: parentCtxt, parser: playlistParser, forwardPlaylist: forwardPlaylist,
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
// @Param Video-Source-Name header string true "Video source name this playlist belongs to"
// @Param MPEG-TS-URI-Prefix header string true "URI prefix for the MPEG-TS segments. The assumption is that all segments listed in the playlist are relative to this prefix."
// @Param playlist body string true "Playlist list content"
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

	// Get video source name
	videoSourceName := r.Header.Get("Video-Source-Name")
	if videoSourceName == "" {
		msg := "request missing Video-Source-Name"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

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
		if len(oneLine) > 0 && oneLine[0] != '\n' {
			playlistRaw = append(playlistRaw, oneLine)
		}
	}
	{
		b := strings.Builder{}
		for idx, oneLine := range playlistRaw {
			b.WriteString(fmt.Sprintf("[%d] '%s'\n", idx, oneLine))
		}
		log.WithFields(logTags).Debugf("Received playlist:\n%s\n", b.String())
	}

	currentTime := time.Now().UTC()

	// Parse the playlist
	playlist, err := h.parser.ParsePlaylist(
		r.Context(), playlistRaw, currentTime, videoSourceName, tsFileURIPrefix,
	)
	if err != nil {
		msg := "unable to parse sent playlist"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
		return
	}

	// Forward the playlist for further processing
	if err := h.forwardPlaylist(h.parentCtxt, playlist, currentTime); err != nil {
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

// =====================================================================================
// Live Stream Video Segment Receiver - Hosted by system control node

// VideoSegmentForwardCB callback signature of function for sending newly received video segments
// onward for processing
type VideoSegmentForwardCB func(
	ctxt context.Context, sourceID string, segment hls.Segment, content []byte,
) error

// SegmentReceiveHandler HLS video segment receiver
//
// This is only meant to be used by the system control node
type SegmentReceiveHandler struct {
	goutils.RestAPIHandler
	parentCtxt     context.Context
	forwardSegment VideoSegmentForwardCB
}

/*
NewSegmentReceiveHandler define a new video segment receiver

	@param parentCtxt context.Context - REST handler parent context
	@param forwardSegment VideoSegmentForwardCB - callback function for sending newly received
	    video segment onward for processing
	@param logConfig common.HTTPRequestLogging - handler log settings
	@returns new SegmentReceiveHandler
*/
func NewSegmentReceiveHandler(
	parentCtxt context.Context,
	forwardSegment VideoSegmentForwardCB,
	logConfig common.HTTPRequestLogging,
) (SegmentReceiveHandler, error) {
	return SegmentReceiveHandler{
		RestAPIHandler: goutils.RestAPIHandler{
			Component: goutils.Component{
				LogTags: log.Fields{"module": "api", "component": "segment-receiver-handler"},
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
		}, parentCtxt: parentCtxt, forwardSegment: forwardSegment,
	}, nil
}

// NewSegment godoc
// @Summary Process new HLS video segment
// @Description Process new HLS video segment
// @tags live,cloud
// @Accept plain
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param Source-ID header string true "Video source ID this segment belongs to"
// @Param Segment-Name header string true "Video segment name"
// @Param Segment-Start-TS header int64 true "Video segment start timestamp - epoch seconds"
// @Param Segment-Length-MSec header int64 true "Video segment duration in millisecond"
// @Param Segment-URI header string true "Video segment URI"
// @Param segmentContent body []byte] true "Video segment content"
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/new-segment [post]
func (h SegmentReceiveHandler) NewSegment(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get video source ID
	videoSourceID := r.Header.Get(ipc.HTTPSegmentForwardHeaderSourceID)
	if videoSourceID == "" {
		msg := fmt.Sprintf("request missing '%s'", ipc.HTTPSegmentForwardHeaderSourceID)
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Get video segment name
	segmentName := r.Header.Get(ipc.HTTPSegmentForwardHeaderName)
	if segmentName == "" {
		msg := fmt.Sprintf("request missing '%s'", ipc.HTTPSegmentForwardHeaderName)
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Get video segment start timestamp
	startTimeRaw := r.Header.Get(ipc.HTTPSegmentForwardHeaderStartTS)
	if startTimeRaw == "" {
		msg := fmt.Sprintf("request missing '%s'", ipc.HTTPSegmentForwardHeaderStartTS)
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}
	startTimeEpoch, err := strconv.ParseInt(startTimeRaw, 10, 64)
	if err != nil {
		msg := fmt.Sprintf(
			"header parameter '%s' is not int64 epoch timestamp",
			ipc.HTTPSegmentForwardHeaderStartTS,
		)
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Get video segment length
	segmentLengthMSecRaw := r.Header.Get(ipc.HTTPSegmentForwardHeaderLength)
	if segmentLengthMSecRaw == "" {
		msg := fmt.Sprintf("request missing '%s'", ipc.HTTPSegmentForwardHeaderLength)
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}
	segmentLengthMSec, err := strconv.ParseInt(segmentLengthMSecRaw, 10, 64)
	if err != nil {
		msg := fmt.Sprintf("header parameter '%s' is not int64", ipc.HTTPSegmentForwardHeaderLength)
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Get video segment URI
	segmentURI := r.Header.Get(ipc.HTTPSegmentForwardHeaderSegURI)
	if segmentURI == "" {
		msg := fmt.Sprintf("request missing '%s'", ipc.HTTPSegmentForwardHeaderSegURI)
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Read the entire segment into memory
	content, err := io.ReadAll(r.Body)
	if err != nil {
		msg := "unable to read video segment data from request"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Process the timestamps - epoch seconds
	startTime := time.Unix(startTimeEpoch, 0)
	segmentLength := time.Millisecond * time.Duration(segmentLengthMSec)
	endTime := startTime.Add(segmentLength)

	// Build segment entry
	segment := hls.Segment{
		Name:      segmentName,
		StartTime: startTime,
		EndTime:   endTime,
		Length:    segmentLength.Seconds(),
		URI:       segmentURI,
	}

	// Record segment
	if err := h.forwardSegment(h.parentCtxt, videoSourceID, segment, content); err != nil {
		msg := "video segment processing failed"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	respCode = http.StatusOK
	response = h.GetStdRESTSuccessMsg(r.Context())
}

// NewSegmentHandler Wrapper around NewSegment
func (h SegmentReceiveHandler) NewSegmentHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.NewSegment(w, r)
	}
}
