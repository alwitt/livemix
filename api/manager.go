package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/control"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

// SystemManagerHandler REST API interface to SystemManager
//
// This is only meant to be used by the control node
type SystemManagerHandler struct {
	goutils.RestAPIHandler
	validate *validator.Validate
	manager  control.SystemManager
}

/*
NewSystemManagerHandler define a new system manager REST API handler

	@param manager control.SystemManager - core system manager
	@param logConfig common.HTTPRequestLogging - handler log settings
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns new SystemManagerHandler
*/
func NewSystemManagerHandler(
	manager control.SystemManager,
	logConfig common.HTTPRequestLogging,
	metrics goutils.HTTPRequestMetricHelper,
) (SystemManagerHandler, error) {
	return SystemManagerHandler{
		RestAPIHandler: goutils.RestAPIHandler{
			Component: goutils.Component{
				LogTags: log.Fields{"module": "api", "component": "system-manager-handler"},
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
		}, validate: validator.New(), manager: manager,
	}, nil
}

// ====================================================================================
// Video Source CRUD

// NewVideoSourceRequest parameters to define a new video source
type NewVideoSourceRequest struct {
	Name             string  `json:"name" validate:"required"`
	TargetSegmentLen int     `json:"segment_len" validate:"required,gte=1"`
	Description      *string `json:"description,omitempty"`
	PlaylistURI      *string `json:"playlist,omitempty" validate:"omitempty,uri"`
}

// VideoSourceInfoResponse response containing information for one video source
type VideoSourceInfoResponse struct {
	goutils.RestAPIBaseResponse
	// Source the video source info
	Source common.VideoSource `json:"source" validate:"required,dive"`
}

// DefineNewVideoSource godoc
// @Summary Define a new video source
// @Description Define a new video source within the system.
// @tags management,cloud
// @Accept json
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param param body NewVideoSourceRequest true "Video source parameters"
// @Success 200 {object} VideoSourceInfoResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source [post]
func (h SystemManagerHandler) DefineNewVideoSource(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	if r.Body == nil {
		msg := "no payload provided to define new video source"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Parse the create parameters
	var params NewVideoSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		msg := "unable to parse new video source parameters from request"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.WithError(err).WithFields(logTags).Error("Request body close error")
		}
	}()

	{
		t, _ := json.Marshal(&params)
		log.
			WithFields(logTags).
			WithField("new-video-source", string(t)).
			Debug("Defining new video source")
	}

	// Validate parameters
	if err := h.validate.Struct(&params); err != nil {
		msg := "messing required values to define new video source"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
		return
	}

	// Define the video source
	entryID, err := h.manager.DefineVideoSource(
		r.Context(), params.Name, params.TargetSegmentLen, params.PlaylistURI, params.Description,
	)
	if err != nil {
		msg := "failed to define new video source"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Read back the video source
	entry, err := h.manager.GetVideoSource(r.Context(), entryID)
	if err != nil {
		msg := "failed to read back the new video source entry"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return new video source
	respCode = http.StatusOK
	response = VideoSourceInfoResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()), Source: entry,
	}
}

// DefineNewVideoSourceHandler Wrapper around DefineNewVideoSource
func (h SystemManagerHandler) DefineNewVideoSourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.DefineNewVideoSource(w, r)
	}
}

// ------------------------------------------------------------------------------------

// ListVideoSources godoc
// @Summary List known video sources
// @Description Fetch list of known video sources in the system
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param source_name query string false "If exist, return this particular video source"
// @Success 200 {object} common.VideoSourceInfoListResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source [get]
func (h SystemManagerHandler) ListVideoSources(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Whether to find a particular video source
	queryParams := r.URL.Query()
	targetSourceName := queryParams.Get("source_name")
	entries := []common.VideoSource{}
	if targetSourceName != "" {
		// Only query for this source
		entry, err := h.manager.GetVideoSourceByName(r.Context(), targetSourceName)
		if err != nil {
			msg := "failed to find specific video source"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
		entries = append(entries, entry)
	} else {
		// Return all sources
		var err error
		entries, err = h.manager.ListVideoSources(r.Context())
		if err != nil {
			msg := "failed to list known video sources"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusInternalServerError
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
			return
		}
	}

	// Return new video source
	respCode = http.StatusOK
	response = common.VideoSourceInfoListResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()), Sources: entries,
	}
}

// ListVideoSourcesHandler Wrapper around ListVideoSources
func (h SystemManagerHandler) ListVideoSourcesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ListVideoSources(w, r)
	}
}

// ------------------------------------------------------------------------------------

// GetVideoSource godoc
// @Summary Fetch video source
// @Description Fetch video source
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param sourceID path string true "Video source ID"
// @Success 200 {object} VideoSourceInfoResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source/{sourceID} [get]
func (h SystemManagerHandler) GetVideoSource(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get video source ID
	vars := mux.Vars(r)
	videoSourceID, ok := vars["sourceID"]
	if !ok {
		msg := "video source ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Get the video source
	entry, err := h.manager.GetVideoSource(r.Context(), videoSourceID)
	if err != nil {
		msg := "failed to fetch video source info"
		log.WithError(err).WithFields(logTags).WithField("video-source", videoSourceID).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return video source
	respCode = http.StatusOK
	response = VideoSourceInfoResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()), Source: entry,
	}
}

// GetVideoSourceHandler Wrapper around GetVideoSource
func (h SystemManagerHandler) GetVideoSourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.GetVideoSource(w, r)
	}
}

// ------------------------------------------------------------------------------------

// DeleteVideoSource godoc
// @Summary Delete a video source
// @Description Delete a video source
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param sourceID path string true "Video source ID"
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source/{sourceID} [delete]
func (h SystemManagerHandler) DeleteVideoSource(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get video source ID
	vars := mux.Vars(r)
	videoSourceID, ok := vars["sourceID"]
	if !ok {
		msg := "video source ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	if err := h.manager.DeleteVideoSource(r.Context(), videoSourceID); err != nil {
		msg := "failed to delete video source"
		log.WithError(err).WithFields(logTags).WithField("video-source", videoSourceID).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return video source
	respCode = http.StatusOK
	response = h.GetStdRESTSuccessMsg(r.Context())
}

// DeleteVideoSourceHandler Wrapper around DeleteVideoSource
func (h SystemManagerHandler) DeleteVideoSourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.DeleteVideoSource(w, r)
	}
}

// ------------------------------------------------------------------------------------

// UpdateVideoSourceName godoc
// @Summary Update a video source's name
// @Description Update a video source's name
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param sourceID path string true "Video source ID"
// @Param new_name query string true "New video source name"
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source/{sourceID}/name [put]
func (h SystemManagerHandler) UpdateVideoSourceName(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get video source ID
	vars := mux.Vars(r)
	videoSourceID, ok := vars["sourceID"]
	if !ok {
		msg := "video source ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Get new video source name
	queryParams := r.URL.Query()
	newName := queryParams.Get("new_name")
	if newName == "" {
		msg := "request did not provide a new video source name"
		log.WithFields(logTags).WithField("video-source", videoSourceID).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Get the video source
	entry, err := h.manager.GetVideoSource(r.Context(), videoSourceID)
	if err != nil {
		msg := "failed to fetch video source info"
		log.WithError(err).WithFields(logTags).WithField("video-source", videoSourceID).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Update info and record
	entry.Name = newName
	if err := h.manager.UpdateVideoSource(r.Context(), entry); err != nil {
		msg := "video source info update failed"
		log.WithError(err).WithFields(logTags).WithField("video-source", videoSourceID).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return video source
	respCode = http.StatusOK
	response = h.GetStdRESTSuccessMsg(r.Context())
}

// UpdateVideoSourceNameHandler Wrapper around UpdateVideoSourceName
func (h SystemManagerHandler) UpdateVideoSourceNameHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.UpdateVideoSourceName(w, r)
	}
}

// ------------------------------------------------------------------------------------

// ChangeSourceStreamingState godoc
// @Summary Change video source streaming state
// @Description Change video source streaming state
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param sourceID path string true "Video source ID"
// @Param new_state query string true "New video source streaming state [true,false]"
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source/{sourceID}/streaming [put]
func (h SystemManagerHandler) ChangeSourceStreamingState(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get video source ID
	vars := mux.Vars(r)
	videoSourceID, ok := vars["sourceID"]
	if !ok {
		msg := "video source ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Get new video streaming state
	queryParams := r.URL.Query()
	newStateVal := queryParams.Get("new_state")
	if newStateVal == "" {
		msg := "request did not provide a new video source streaming state"
		log.WithFields(logTags).WithField("video-source", videoSourceID).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}
	var newState int
	if newStateVal == "true" {
		newState = 1
	} else {
		newState = -1
	}

	// Change the streaming state
	err := h.manager.ChangeVideoSourceStreamState(r.Context(), videoSourceID, newState)
	if err != nil {
		msg := "video source streaming state update failed"
		log.WithError(err).WithFields(logTags).WithField("video-source", videoSourceID).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return video source
	respCode = http.StatusOK
	response = h.GetStdRESTSuccessMsg(r.Context())
}

// ChangeSourceStreamingStateHandler Wrapper around ChangeSourceStreamingState
func (h SystemManagerHandler) ChangeSourceStreamingStateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ChangeSourceStreamingState(w, r)
	}
}

// ------------------------------------------------------------------------------------

// ExchangeVideoSourceStatusInfo godoc
// @Summary Edge and Control Exchange Video Source State
// @Description Process video source status report from an edge node, and return the
// current state of the video source according to control.
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param sourceID path string true "Video source ID"
// @Param param body common.VideoSourceStatusReport true "Video source status report"
// @Success 200 {object} common.VideoSourceCurrentStateResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source/{sourceID}/status [post]
func (h SystemManagerHandler) ExchangeVideoSourceStatusInfo(
	w http.ResponseWriter, r *http.Request,
) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get video source ID
	vars := mux.Vars(r)
	videoSourceID, ok := vars["sourceID"]
	if !ok {
		msg := "video source ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	if r.Body == nil {
		msg := "no payload provided to for status report"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Parse the status report
	var params common.VideoSourceStatusReport
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		msg := "unable to parse video source status report from request"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.WithError(err).WithFields(logTags).Error("Request body close error")
		}
	}()

	// Validate parameters
	if err := h.validate.Struct(&params); err != nil {
		msg := "messing required values to update video source status"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
		return
	}

	// Get the video source
	entry, err := h.manager.GetVideoSource(r.Context(), videoSourceID)
	if err != nil {
		msg := "failed to fetch video source info"
		log.WithError(err).WithFields(logTags).WithField("video-source", videoSourceID).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Record new video source status
	if err := h.manager.UpdateVideoSourceStatus(
		r.Context(), entry.ID, params.RequestResponseTargetID, params.LocalTimestamp,
	); err != nil {
		msg := "failed to update video source status"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Get active recordings of video source
	recordings, err := h.manager.ListRecordingSessionsOfSource(r.Context(), videoSourceID, true)
	if err != nil {
		msg := "failed to list active recording sessions of source"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return the recording session
	respCode = http.StatusOK
	response = common.VideoSourceCurrentStateResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()),
		Source:              entry,
		Recordings:          recordings,
	}
}

// ExchangeVideoSourceStatusInfoHandler Wrapper around ExchangeVideoSourceStatusInfo
func (h SystemManagerHandler) ExchangeVideoSourceStatusInfoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ExchangeVideoSourceStatusInfo(w, r)
	}
}

// ------------------------------------------------------------------------------------

// StartNewRecordingRequest parameters to start a new video recording
type StartNewRecordingRequest struct {
	Alias       *string `json:"alias,omitempty"`
	Description *string `json:"description,omitempty"`
	StartTime   int64   `json:"start_time_epoch,omitempty"`
}

// RecordingSessionResponse response containing information for one recording session
type RecordingSessionResponse struct {
	goutils.RestAPIBaseResponse
	// Recording video recording session info
	Recording common.Recording `json:"recording" validate:"required,dive"`
}

// StartNewRecording godoc
// @Summary Start video recording
// @Description Starting a new video recording session on a particular video source
// @tags management,cloud
// @Accept json
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param sourceID path string true "Video source ID"
// @Param requestPayload body StartNewRecordingRequest false "Recording session parameters"
// @Success 200 {object} RecordingSessionResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source/{sourceID}/recording [post]
func (h SystemManagerHandler) StartNewRecording(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get video source ID
	vars := mux.Vars(r)
	videoSourceID, ok := vars["sourceID"]
	if !ok {
		msg := "video source ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	startTime := time.Now().UTC()
	var alias, description *string

	var paramRaw []byte
	if r.Body != nil {
		defer func() {
			if err := r.Body.Close(); err != nil {
				log.WithError(err).WithFields(logTags).Error("Request body close error")
			}
		}()
		if t, err := io.ReadAll(r.Body); err == nil {
			paramRaw = t
		}
	}

	if len(paramRaw) > 0 {
		// Parse the start parameters
		var params StartNewRecordingRequest
		if err := json.Unmarshal(paramRaw, &params); err != nil {
			msg := "unable to parse new recording session parameters from request"
			log.WithError(err).WithFields(logTags).Error(msg)
			respCode = http.StatusBadRequest
			response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, err.Error())
			return
		}

		// Convert epoch time to time.Time
		startTime = time.Unix(params.StartTime, 0)

		alias = params.Alias
		description = params.Description
	}

	log.
		WithFields(logTags).
		WithField("source-id", videoSourceID).
		Debug("Starting new video recording session")

	recordingID, err := h.manager.DefineRecordingSession(
		r.Context(), videoSourceID, alias, description, startTime,
	)
	if err != nil {
		msg := "failed to start new video recording session"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Read back the recording session
	entry, err := h.manager.GetRecordingSession(r.Context(), recordingID)
	if err != nil {
		msg := "failed to read back the recording session"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return the recording session
	respCode = http.StatusOK
	response = RecordingSessionResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()), Recording: entry,
	}
}

// StartNewRecordingHandler Wrapper around StartNewRecording
func (h SystemManagerHandler) StartNewRecordingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.StartNewRecording(w, r)
	}
}

// ------------------------------------------------------------------------------------

// ListRecordings godoc
// @Summary Fetch recordings
// @Description Fetch video recording sessions
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Success 200 {object} common.RecordingSessionListResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/recording [get]
func (h SystemManagerHandler) ListRecordings(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	recordings, err := h.manager.ListRecordingSessions(r.Context())
	if err != nil {
		msg := "failed to list all recording sessions"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return the recording session
	respCode = http.StatusOK
	response = common.RecordingSessionListResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()), Recordings: recordings,
	}
}

// ListRecordingsHandler Wrapper around ListRecordings
func (h SystemManagerHandler) ListRecordingsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ListRecordings(w, r)
	}
}

// ------------------------------------------------------------------------------------

// ListRecordingsOfSource godoc
// @Summary Fetch recordings of source
// @Description Fetch video recording sessions associated with a video source
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param sourceID path string true "Video source ID"
// @Param only_active query string false "If exist, only list active recording sessions"
// @Success 200 {object} RecordingSessionListResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source/{sourceID}/recording [get]
func (h SystemManagerHandler) ListRecordingsOfSource(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get video source ID
	vars := mux.Vars(r)
	videoSourceID, ok := vars["sourceID"]
	if !ok {
		msg := "video source ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Whether to list only active recordings
	queryParams := r.URL.Query()
	onlyActive := queryParams.Get("only_active") != ""

	recordings, err := h.manager.ListRecordingSessionsOfSource(r.Context(), videoSourceID, onlyActive)
	if err != nil {
		msg := "failed to list recording sessions of source"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return the recording session
	respCode = http.StatusOK
	response = common.RecordingSessionListResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()), Recordings: recordings,
	}
}

// ListRecordingsOfSourceHandler Wrapper around ListRecordingsOfSource
func (h SystemManagerHandler) ListRecordingsOfSourceHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ListRecordingsOfSource(w, r)
	}
}

// ------------------------------------------------------------------------------------

// GetRecording godoc
// @Summary Get recording session
// @Description Get the video recording sessions
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param recordingID path string true "Video recording session ID"
// @Success 200 {object} RecordingSessionResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/recording/{recordingID} [get]
func (h SystemManagerHandler) GetRecording(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get recording ID
	vars := mux.Vars(r)
	recordingID, ok := vars["recordingID"]
	if !ok {
		msg := "recording session ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	entry, err := h.manager.GetRecordingSession(r.Context(), recordingID)
	if err != nil {
		msg := "failed to read back the recording session"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return the recording session
	respCode = http.StatusOK
	response = RecordingSessionResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()), Recording: entry,
	}
}

// GetRecordingHandler Wrapper around GetRecording
func (h SystemManagerHandler) GetRecordingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.GetRecording(w, r)
	}
}

// ------------------------------------------------------------------------------------

// RecordingSegmentListResponse response containing set of segment associated with a recording
type RecordingSegmentListResponse struct {
	goutils.RestAPIBaseResponse
	// Segments video segments associated with a recording session
	Segments []common.VideoSegment `json:"segments" validate:"required,dive"`
}

// ListSegmentsOfRecording godoc
// @Summary Get segments associated with recording
// @Description Get the video segments associated with a recording session
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param recordingID path string true "Video recording session ID"
// @Success 200 {object} RecordingSegmentListResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/recording/{recordingID}/segment [get]
func (h SystemManagerHandler) ListSegmentsOfRecording(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get recording ID
	vars := mux.Vars(r)
	recordingID, ok := vars["recordingID"]
	if !ok {
		msg := "recording session ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	segments, err := h.manager.ListAllSegmentsOfRecording(r.Context(), recordingID)
	if err != nil {
		msg := "failed to read recording session segments"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return the recording session
	respCode = http.StatusOK
	response = RecordingSegmentListResponse{
		RestAPIBaseResponse: h.GetStdRESTSuccessMsg(r.Context()), Segments: segments,
	}
}

// ListSegmentsOfRecordingHandler Wrapper around ListSegmentsOfRecording
func (h SystemManagerHandler) ListSegmentsOfRecordingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ListSegmentsOfRecording(w, r)
	}
}

// ------------------------------------------------------------------------------------

// StopRecording godoc
// @Summary Stop recording session
// @Description Stop the video recording sessions
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param recordingID path string true "Video recording session ID"
// @Param force query string false "If exist, will complete operation regardless of whether the video source is accepting inbound requests."
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/recording/{recordingID}/stop [post]
func (h SystemManagerHandler) StopRecording(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get recording ID
	vars := mux.Vars(r)
	recordingID, ok := vars["recordingID"]
	if !ok {
		msg := "recording session ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Whether to force request through
	queryParams := r.URL.Query()
	willForce := queryParams.Get("force") != ""

	currentTime := time.Now().UTC()

	err := h.manager.MarkEndOfRecordingSession(r.Context(), recordingID, currentTime, willForce)
	if err != nil {
		msg := "failed to stop recording session"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	respCode = http.StatusOK
	response = h.GetStdRESTSuccessMsg(r.Context())
}

// StopRecordingHandler Wrapper around StopRecording
func (h SystemManagerHandler) StopRecordingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.StopRecording(w, r)
	}
}

// ------------------------------------------------------------------------------------

// DeleteRecording godoc
// @Summary Delete recording session
// @Description Delete the video recording sessions
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Param recordingID path string true "Video recording session ID"
// @Param force query string false "If exist, will complete operation regardless of whether the video source is accepting inbound requests."
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/recording/{recordingID} [delete]
func (h SystemManagerHandler) DeleteRecording(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	// Get recording ID
	vars := mux.Vars(r)
	recordingID, ok := vars["recordingID"]
	if !ok {
		msg := "recording session ID missing from request URL"
		log.WithFields(logTags).Error(msg)
		respCode = http.StatusBadRequest
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusBadRequest, msg, msg)
		return
	}

	// Whether to force request through
	queryParams := r.URL.Query()
	willForce := queryParams.Get("force") != ""

	if err := h.manager.DeleteRecordingSession(r.Context(), recordingID, willForce); err != nil {
		msg := "failed to delete recording session"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	respCode = http.StatusOK
	response = h.GetStdRESTSuccessMsg(r.Context())
}

// DeleteRecordingHandler Wrapper around DeleteRecording
func (h SystemManagerHandler) DeleteRecordingHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.DeleteRecording(w, r)
	}
}

// ====================================================================================
// Utilities

// SystemManagerLivenessHandler liveness REST API interface for SystemManager
type SystemManagerLivenessHandler struct {
	goutils.RestAPIHandler
	manager control.SystemManager
}

/*
NewSystemManagerLivenessHandler define a new system manager liveness REST API handler

	@param manager control.SystemManager - core system manager
	@param logConfig common.HTTPRequestLogging - handler log settings
	@returns new SystemManagerLivenessHandler
*/
func NewSystemManagerLivenessHandler(
	manager control.SystemManager, logConfig common.HTTPRequestLogging,
) (SystemManagerLivenessHandler, error) {
	return SystemManagerLivenessHandler{
		RestAPIHandler: goutils.RestAPIHandler{
			Component: goutils.Component{
				LogTags: log.Fields{"module": "api", "component": "system-manager-liveness-handler"},
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
		}, manager: manager,
	}, nil
}

// Alive godoc
// @Summary System Manager API liveness check
// @Description Will return success to indicate system manager REST API module is live
// @tags util,management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/alive [get]
func (h SystemManagerLivenessHandler) Alive(w http.ResponseWriter, r *http.Request) {
	logTags := h.GetLogTagsForContext(r.Context())
	if err := h.WriteRESTResponse(
		w, http.StatusOK, h.GetStdRESTSuccessMsg(r.Context()), nil,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to form response")
	}
}

// AliveHandler Wrapper around Alive
func (h SystemManagerLivenessHandler) AliveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.Alive(w, r)
	}
}

// -----------------------------------------------------------------------

// Ready godoc
// @Summary System Manager API readiness check
// @Description Will return success if system manager REST API module is ready for use
// @tags util,management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Success 200 {object} goutils.RestAPIBaseResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/ready [get]
func (h SystemManagerLivenessHandler) Ready(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()
	if err := h.manager.Ready(r.Context()); err != nil {
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(
			r.Context(), http.StatusInternalServerError, "not ready", err.Error(),
		)
	} else {
		respCode = http.StatusOK
		response = h.GetStdRESTSuccessMsg(r.Context())
	}
}

// ReadyHandler Wrapper around Alive
func (h SystemManagerLivenessHandler) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.Ready(w, r)
	}
}
