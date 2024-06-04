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
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/edge"
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
	parentCtxt              context.Context
	parser                  hls.PlaylistParser
	forwardPlaylist         PlaylistForwardCB
	defaultSourceName       string
	defaultSegmentURIPrefix *string
}

/*
NewPlaylistReceiveHandler define a new HLS playlist receiver

	@param parentCtxt context.Context - REST handler parent context
	@param playlistParser hls.PlaylistParser - playlist parser object
	@param forwardPlaylist - callback function for sending newly received HLS playlist
	    onward for processing
	@param defaultSourceName string - if video source name not provided, use this as the default
	    name for the playlist.
	@param defaultSegmentURIPrefix *string - if video segment URI prefix is not provided, use this
	    as the default prefix.
	@param logConfig common.HTTPRequestLogging - handler log settings
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns new PlaylistReceiveHandler
*/
func NewPlaylistReceiveHandler(
	parentCtxt context.Context,
	playlistParser hls.PlaylistParser,
	forwardPlaylist PlaylistForwardCB,
	defaultSourceName string,
	defaultSegmentURIPrefix *string,
	logConfig common.HTTPRequestLogging,
	metrics goutils.HTTPRequestMetricHelper,
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
			LogLevel:      logConfig.LogLevel,
			MetricsHelper: metrics,
		},
		parentCtxt:              parentCtxt,
		parser:                  playlistParser,
		forwardPlaylist:         forwardPlaylist,
		defaultSourceName:       defaultSourceName,
		defaultSegmentURIPrefix: defaultSegmentURIPrefix,
	}, nil
}

// NewPlaylist godoc
// @Summary Process new HLS playlist
// @Description Receives new HLS playlist for processing. The sender should provide the
// location where the MPEG-TS segment are stored as a URI prefix. The HLS playlist monitor
// will use that prefix to pull the segments into the system. A default is used if none provided.
// @tags edge
// @Accept plain
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param Video-Source-Name header string false "Video source name this playlist belongs to"
// @Param MPEG-TS-URI-Prefix header string false "URI prefix for the MPEG-TS segments. The assumption is that all segments listed in the playlist are relative to this prefix."
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
		videoSourceName = h.defaultSourceName
	}

	// Get the MPEG-TS prefix
	tsFileURIPrefix := r.Header.Get("MPEG-TS-URI-Prefix")
	if tsFileURIPrefix == "" {
		if h.defaultSegmentURIPrefix == nil {
			msg := "request missing MPEG-TS-URI-Prefix"
			log.WithFields(logTags).Error(msg)
			respCode = http.StatusBadRequest
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
			return
		}
		tsFileURIPrefix = *h.defaultSegmentURIPrefix
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
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns new SegmentReceiveHandler
*/
func NewSegmentReceiveHandler(
	parentCtxt context.Context,
	forwardSegment VideoSegmentForwardCB,
	logConfig common.HTTPRequestLogging,
	metrics goutils.HTTPRequestMetricHelper,
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
			LogLevel:      logConfig.LogLevel,
			MetricsHelper: metrics,
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
// @Param segmentContent body []byte true "Video segment content"
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

// =====================================================================================
// Edge Node Utilities

// EdgeNodeLivenessHandler liveness REST API interface for edge node
type EdgeNodeLivenessHandler struct {
	goutils.RestAPIHandler
	self    common.VideoSourceConfig
	dbConns db.ConnectionManager
	manager edge.VideoSourceOperator
}

/*
NewEdgeNodeLivenessHandler define a new edge node liveness REST API handler

	@param self common.VideoSourceConfig - video source entity parameter
	@param dbConns db.ConnectionManager - DB connection manager
	@param manager edge.VideoSourceOperator - video source manager
	@param logConfig common.HTTPRequestLogging - handler log settings
	@return new EdgeNodeLivenessHandler
*/
func NewEdgeNodeLivenessHandler(
	self common.VideoSourceConfig,
	dbConns db.ConnectionManager,
	manager edge.VideoSourceOperator,
	logConfig common.HTTPRequestLogging,
) (EdgeNodeLivenessHandler, error) {
	return EdgeNodeLivenessHandler{
		RestAPIHandler: goutils.RestAPIHandler{
			Component: goutils.Component{
				LogTags: log.Fields{"module": "api", "component": "edge-node-liveness-handler"},
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
			LogLevel: logConfig.HealthLogLevel,
		}, self: self, dbConns: dbConns, manager: manager,
	}, nil
}

// -----------------------------------------------------------------------

// Alive godoc
// @Summary Edge Node liveness check
// @Description Will return success to indicate Edge Node is live
// @tags util,management,edge
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/alive [get]
func (h EdgeNodeLivenessHandler) Alive(w http.ResponseWriter, r *http.Request) {
	logTags := h.GetLogTagsForContext(r.Context())
	if err := h.WriteRESTResponse(
		w, http.StatusOK, h.GetStdRESTSuccessMsg(r.Context()), nil,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to form response")
	}
}

// AliveHandler Wrapper around Alive
func (h EdgeNodeLivenessHandler) AliveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.Alive(w, r)
	}
}

// -----------------------------------------------------------------------

// Ready godoc
// @Summary Edge Node readiness check
// @Description Will return success to indicate Edge Node is ready
// @tags util,management,edge
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/ready [get]
func (h EdgeNodeLivenessHandler) Ready(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	db := h.dbConns.NewPersistanceManager()
	defer db.Close()

	if err := db.Ready(r.Context()); err != nil {
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(
			r.Context(), http.StatusInternalServerError, "not ready", err.Error(),
		)
		return
	}

	timestamp := time.Now().UTC()
	updatedEntity, err := db.GetVideoSourceByName(r.Context(), h.self.Name)
	if err != nil {
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(
			r.Context(), http.StatusInternalServerError, "local cache not setup", err.Error(),
		)
		return
	}

	timeToLastUpdate := timestamp.Sub(updatedEntity.SourceLocalTime)
	if timeToLastUpdate > h.self.StatusReportInt()*2 {
		err := fmt.Errorf("status report transmission passed deadline")
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(
			r.Context(), http.StatusInternalServerError, "not ready", err.Error(),
		)
		return
	}

	if cacheCount, err := h.manager.CacheEntryCount(r.Context()); err != nil || cacheCount == 0 {
		err := fmt.Errorf("video segment cache is empty or not functional")
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(
			r.Context(), http.StatusInternalServerError, "not ready", err.Error(),
		)
		return
	}

	respCode = http.StatusOK
	response = h.GetStdRESTSuccessMsg(r.Context())
}

// ReadyHandler Wrapper around Ready
func (h EdgeNodeLivenessHandler) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.Ready(w, r)
	}
}
