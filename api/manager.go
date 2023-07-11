package api

import (
	"encoding/json"
	"net/http"

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
	@returns new SystemManagerHandler
*/
func NewSystemManagerHandler(
	manager control.SystemManager, logConfig common.HTTPRequestLogging,
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
			LogLevel: logConfig.LogLevel,
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

// VideoSourceInfoListResponse response containing list of video sources
type VideoSourceInfoListResponse struct {
	goutils.RestAPIBaseResponse
	// Sources list of video source infos
	Sources []common.VideoSource `json:"sources" validate:"required,gte=1,dive"`
}

// ListVideoSources godoc
// @Summary List known video sources
// @Description Fetch list of known video sources in the system
// @tags management,cloud
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Success 200 {object} VideoSourceInfoListResponse "success"
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

	entries, err := h.manager.ListVideoSources(r.Context())
	if err != nil {
		msg := "failed to list known video sources"
		log.WithError(err).WithFields(logTags).Error(msg)
		respCode = http.StatusInternalServerError
		response = h.GetStdRESTErrorMsg(r.Context(), http.StatusInternalServerError, msg, err.Error())
		return
	}

	// Return new video source
	respCode = http.StatusOK
	response = VideoSourceInfoListResponse{
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

// ====================================================================================
// Utilities

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
func (h SystemManagerHandler) Alive(w http.ResponseWriter, r *http.Request) {
	logTags := h.GetLogTagsForContext(r.Context())
	if err := h.WriteRESTResponse(
		w, http.StatusOK, h.GetStdRESTSuccessMsg(r.Context()), nil,
	); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to form response")
	}
}

// AliveHandler Wrapper around Alive
func (h SystemManagerHandler) AliveHandler() http.HandlerFunc {
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
func (h SystemManagerHandler) Ready(w http.ResponseWriter, r *http.Request) {
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
func (h SystemManagerHandler) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.Ready(w, r)
	}
}
