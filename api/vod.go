package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/vod"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

// VODHandler HLS VOD handler
type VODHandler struct {
	goutils.RestAPIHandler
	validate *validator.Validate
	dbConns  db.ConnectionManager
	manager  vod.PlaylistManager
}

/*
NewVODHandler define a new HLS VOD handler

	@param dbConns db.ConnectionManager - DB connection manager
	@param manager vod.PlaylistManager - video playlist manager
	@param logConfig common.HTTPRequestLogging - handler log settings
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns new LiveStreamHandler
*/
func NewVODHandler(
	dbConns db.ConnectionManager,
	manager vod.PlaylistManager,
	logConfig common.HTTPRequestLogging,
	metrics goutils.HTTPRequestMetricHelper,
) (VODHandler, error) {
	return VODHandler{
		RestAPIHandler: goutils.RestAPIHandler{
			Component: goutils.Component{
				LogTags: log.Fields{"module": "api", "component": "vod-handler"},
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
		}, validate: validator.New(), dbConns: dbConns, manager: manager,
	}, nil
}

const (
	vodHandlerOpModeLiveStream = "FETCH_LIVE_STREAM_FILE"
	vodHandlerOpModeRecording  = "FETCH_RECORDING_FILE"
)

func (h VODHandler) getVideoFiles(w http.ResponseWriter, r *http.Request, opMode string) {
	var respCode int
	var response interface{}
	bypassDefaultResponseWriter := false
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if !bypassDefaultResponseWriter {
			if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
				log.WithError(err).WithFields(logTags).Error("Failed to form response")
			}
		}
	}()

	liveStream := opMode != vodHandlerOpModeRecording

	dbClient := h.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	vars := mux.Vars(r)
	var videoSource common.VideoSource
	var recording common.Recording

	if liveStream {
		// Live stream mode
		var err error
		videoSourceID, ok := vars["videoSourceID"]
		if !ok {
			msg := "no video source ID provided"
			log.WithFields(logTags).Error(msg)
			respCode = http.StatusBadRequest
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
			return
		}
		// Get the video source
		videoSource, err = dbClient.GetVideoSource(r.Context(), videoSourceID)
		if err != nil {
			msg := "Unknown video source"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusBadRequest
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
			return
		}
	} else {
		// Recording mode
		var err error
		recordingID, ok := vars["recordingID"]
		if !ok {
			msg := "no video recording ID provided"
			log.WithFields(logTags).Error(msg)
			respCode = http.StatusBadRequest
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
			return
		}
		// Get the recording
		recording, err = dbClient.GetRecordingSession(r.Context(), recordingID)
		if err != nil {
			msg := "Unknown video recording"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusBadRequest
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
			return
		}
	}

	// Get the target name
	fileName, ok := vars["fileName"]
	if !ok {
		msg := "no target file provided"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	log.WithFields(logTags).WithField("target-file", fileName).Debug("User requested file")

	currentTime := time.Now().UTC()

	// Based on the target file name extension
	fileNamePieces := strings.Split(fileName, ".")
	switch strings.ToLower(fileNamePieces[len(fileNamePieces)-1]) {
	// HLS Playlist file
	case "m3u8":
		// Get the current playlist
		var newPlaylist hls.Playlist
		var err error
		var continuous bool
		if liveStream {
			// live stream mode
			newPlaylist, err = h.manager.GetLiveStreamPlaylist(
				r.Context(), videoSource, currentTime, true,
			)
			continuous = true
		} else {
			// recording mode
			newPlaylist, err = h.manager.GetRecordingStreamPlaylist(r.Context(), recording)
			continuous = false
		}
		if err != nil {
			msg := "Failed to construct the playlist"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
		// Serialize the newPlaylistStr for return
		newPlaylistStr, err := newPlaylist.String(continuous)
		if err != nil {
			msg := "Playlist serialization failed"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
		log.WithFields(logTags).Debugf("New playlist:\n%s\n", newPlaylistStr)
		// Send it back
		written, err := w.Write([]byte(newPlaylistStr))
		if err != nil {
			msg := "Write response failure"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
		if written != len(newPlaylistStr) {
			msg := fmt.Sprintf(
				"Write response length does not match expectation: %d =/= %d", written, len(newPlaylistStr),
			)
			log.WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, msg)
			return
		}
		bypassDefaultResponseWriter = true

	// MPEG-TS Video Segment
	case "ts":
		// Find the segment entry from the system
		var segmentEntry common.VideoSegment
		var err error
		if liveStream {
			// live stream mode
			segmentEntry, err = dbClient.GetLiveStreamSegmentByName(r.Context(), fileName)
		} else {
			// recording mode
			segmentEntry, err = dbClient.GetRecordingSegmentByName(r.Context(), fileName)
		}
		if err != nil {
			msg := "Unknown segment name"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusBadRequest
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
			return
		}
		// Get the segment
		segment, err := h.manager.GetSegment(r.Context(), segmentEntry)
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
		bypassDefaultResponseWriter = true

	// Everything else
	default:
		err := fmt.Errorf("target file '%s' has unsupported file extension", fileName)
		msg := "Unsupported target filename"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
	}
}

// GetLiveStreamVideoFilesHandler Wrapper around getVideoFiles in live stream mode
// GetLiveStreamVideoFiles godoc
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
func (h VODHandler) GetLiveStreamVideoFilesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.getVideoFiles(w, r, vodHandlerOpModeLiveStream)
	}
}

// GetRecordingVideoFilesHandler Wrapper around getVideoFiles in recording mode
// GetRecordingVideoFiles godoc
// @Summary Query files for a HLS recording
// @Description Query files for a HLS recording, which include both the `m3u8` playlist
// file and `ts` MPEG-TS file.
// @tags vod,recording,edge,cloud
// @Produce plain
// @Param X-Request-ID header string false "Request ID"
// @Param recordingID path string true "Video recording ID"
// @Param fileName path string true "Target file name"
// @Success 200 {object} []byte "HLS m3u8 playlist / MPEG-TS"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/vod/recording/{recordingID}/{fileName} [get]
func (h VODHandler) GetRecordingVideoFilesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.getVideoFiles(w, r, vodHandlerOpModeRecording)
	}
}
