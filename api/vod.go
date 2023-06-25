package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/vod"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

// LiveStreamHandler HLS live stream hosting handler
type LiveStreamHandler struct {
	goutils.RestAPIHandler
	validate  *validator.Validate
	dbClient  db.PersistenceManager
	playlists vod.PlaylistBuilder
	segments  vod.SegmentManager
}

/*
NewLiveStreamHandler define a new HLS live stream hosting handler

	@param dbClient db.PersistenceManager - DB access client
	@param playlists vod.PlaylistBuilder - support playlist builder
	@param segments vod.SegmentManager - support playlist segment manager
	@param logConfig common.HTTPRequestLogging - handler log settings
	@returns new LiveStreamHandler
*/
func NewLiveStreamHandler(
	dbClient db.PersistenceManager,
	playlists vod.PlaylistBuilder,
	segments vod.SegmentManager,
	logConfig common.HTTPRequestLogging,
) (LiveStreamHandler, error) {
	return LiveStreamHandler{
		RestAPIHandler: goutils.RestAPIHandler{
			Component: goutils.Component{
				LogTags: log.Fields{"module": "api", "component": "live-stream-handler"},
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
		}, validate: validator.New(), dbClient: dbClient, playlists: playlists, segments: segments,
	}, nil
}

// GetVideoFiles godoc
// @Summary Query files for a HLS live stream
// @Description Query files for a HLS live stream, which include both the `m3u8` playlist
// file and `ts` MPEG-TS file.
// @tags vod,live,edge,cloud
// @Produce plain
// @Param X-Request-ID header string false "Request ID"
// @Param videoSourceID path string true "Video source ID"
// @Param fileName path string true "Target file name"
// @Success 200 {object} []byte "HLS m3u8 playlist / MPEG-TS"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/vod/live/{videoSourceID}/{fileName} [get]
func (h LiveStreamHandler) GetVideoFiles(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	vars := mux.Vars(r)
	videoSourceID, ok := vars["videoSourceID"]
	if !ok {
		msg := "no video source ID provided"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}
	fileName, ok := vars["fileName"]
	if !ok {
		msg := "no target file provided"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	log.WithFields(logTags).WithField("target-file", fileName).Debug("User requested file")

	// Verify the video source is valid
	videoSource, err := h.dbClient.GetVideoSource(r.Context(), videoSourceID)
	if err != nil {
		msg := "Unknown video source"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
		return
	}

	currentTime := time.Now().UTC()

	// Based on the target file name extension
	fileNamePieces := strings.Split(fileName, ".")
	switch strings.ToLower(fileNamePieces[len(fileNamePieces)-1]) {
	// HLS Playlist file
	case "m3u8":
		// Get the current playlist
		currentPlaylist, err := h.playlists.GetLiveStreamPlaylist(r.Context(), videoSource, currentTime)
		if err != nil {
			msg := "Failed to construct the playlist"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
		// Serialize the playlist for return
		playlist, err := currentPlaylist.String()
		if err != nil {
			msg := "Playlist serialization failed"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
		// Send it back
		if _, err := w.Write([]byte(playlist)); err != nil {
			msg := "Write response failure"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}

	// MPEG-TS Video Segment
	case "ts":
		// Find the segment entry from the system
		segmentEntry, err := h.dbClient.GetLiveStreamSegmentByName(r.Context(), fileName)
		if err != nil {
			msg := "Unknown segment name"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusBadRequest
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
			return
		}
		// Get the segment
		segment, err := h.segments.GetSegment(r.Context(), segmentEntry)
		if err != nil {
			msg := "Failed to find segment"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
		log.WithFields(logTags).Debugf("Response length %d", len(segment))
		// Send it back
		written, err := w.Write(segment)
		if err != nil {
			msg := "Write response failure"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
		if written != len(segment) {
			msg := fmt.Sprintf(
				"Write response length does not match expectation: %d =/= %d", written, len(segment),
			)
			log.WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, msg)
			return
		}

	// Everything else
	default:
		err := fmt.Errorf("target file '%s' has unsupported file extension", fileName)
		msg := "Unsupported target filename"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
	}
}

// GetVideoFilesHandler Wrapper around GetVideoFiles
func (h LiveStreamHandler) GetVideoFilesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.GetVideoFiles(w, r)
	}
}
