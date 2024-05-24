package api

import (
	"net/http"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/apex/log"
)

// EdgeAPIHandler REST API interface for basic information retrieval from the edge node
type EdgeAPIHandler struct {
	goutils.RestAPIHandler
	dbConns db.ConnectionManager
}

/*
NewEdgeAPIHandler define a new edge API handler

	@param dbConns db.ConnectionManager - DB connection manager
	@param logConfig common.HTTPRequestLogging - handler log settings
	@param metrics goutils.HTTPRequestMetricHelper - metric collection agent
	@returns new EdgeAPIHandler
*/
func NewEdgeAPIHandler(
	dbConns db.ConnectionManager,
	logConfig common.HTTPRequestLogging,
	metrics goutils.HTTPRequestMetricHelper,
) (EdgeAPIHandler, error) {
	return EdgeAPIHandler{
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
			LogLevel:      logConfig.LogLevel,
			MetricsHelper: metrics,
		}, dbConns: dbConns,
	}, nil
}

// ListVideoSources godoc
// @Summary List known video sources
// @Description Fetch list of known video sources on this edge
// @tags management,edge
// @Produce json
// @Param X-Request-ID header string false "Request ID"
// @Success 200 {object} VideoSourceInfoListResponse "success"
// @Failure 400 {object} goutils.RestAPIBaseResponse "error"
// @Failure 403 {object} goutils.RestAPIBaseResponse "error"
// @Failure 404 {string} string "error"
// @Failure 500 {object} goutils.RestAPIBaseResponse "error"
// @Router /v1/source [get]
func (h EdgeAPIHandler) ListVideoSources(w http.ResponseWriter, r *http.Request) {
	var respCode int
	var response interface{}
	logTags := h.GetLogTagsForContext(r.Context())
	defer func() {
		if err := h.WriteRESTResponse(w, respCode, response, nil); err != nil {
			log.WithError(err).WithFields(logTags).Error("Failed to form response")
		}
	}()

	dbClient := h.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	entries, err := dbClient.ListVideoSources(r.Context())
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
func (h EdgeAPIHandler) ListVideoSourcesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.ListVideoSources(w, r)
	}
}
