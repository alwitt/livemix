package edge

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/go-resty/resty/v2"
	"github.com/oklog/ulid/v2"
)

// restControlRequestClientImpl implements ControlRequestClient
type restControlRequestClientImpl struct {
	goutils.Component
	ctrlBaseURI     *url.URL
	requestIDHeader string
	client          *resty.Client
	validator       *validator.Validate
}

/*
NewRestControlRequestClient define a new edge to control request-response client based on REST

	@param ctxt context.Context - execution context
	@param controllerBaseURI *url.URL - control node base URL
	@param requestIDHeader string - HTTP header to set for the request ID
	@param httpClient *resty.Client - HTTP client to use
	@return new client
*/
func NewRestControlRequestClient(
	ctxt context.Context,
	controllerBaseURI *url.URL,
	requestIDHeader string,
	httpClient *resty.Client,
) (ControlRequestClient, error) {
	logTags := log.Fields{
		"module":    "api",
		"component": "edge-to-control-rest-client",
		"instance":  controllerBaseURI.String(),
	}

	instance := &restControlRequestClientImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		ctrlBaseURI:     controllerBaseURI,
		requestIDHeader: requestIDHeader,
		client:          httpClient,
		validator:       validator.New(),
	}

	return instance, nil
}

func (c *restControlRequestClientImpl) InstallReferenceToManager(_ VideoSourceOperator) {}

func (c *restControlRequestClientImpl) GetVideoSourceInfo(
	ctxt context.Context, sourceName string,
) (common.VideoSource, error) {
	logTags := c.GetLogTagsForContext(ctxt)

	reqID := ulid.Make().String()

	requestURL := c.ctrlBaseURI.JoinPath("/v1/source")
	resp, err := c.client.R().
		// Set request header for segment reception
		SetHeader(c.requestIDHeader, reqID).
		// Set query parameters
		SetQueryParam("source_name", sourceName).
		// Set response payload
		SetResult(&common.VideoSourceInfoListResponse{}).
		// Setup error parsing
		SetError(goutils.RestAPIBaseResponse{}).
		Get(requestURL.String())

	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source", sourceName).
			Error("Video source look up request failed on call")
		return common.VideoSource{}, err
	}

	if !resp.IsSuccess() {
		respError := resp.Error().(*goutils.RestAPIBaseResponse)
		var err error
		if respError.Error != nil {
			err = fmt.Errorf("%s", respError.Error.Detail)
		} else {
			err = fmt.Errorf("status code %d", resp.StatusCode())
		}
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source", sourceName).
			WithField("outbound-request-id", reqID).
			Error("Video source look up failed")
		return common.VideoSource{}, err
	}

	// Process the response
	videoSourceList, ok := resp.Result().(*common.VideoSourceInfoListResponse)
	if !ok {
		rawResp := string(resp.Body())
		err := fmt.Errorf("failed to parse video source look up response")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source", sourceName).
			WithField("outbound-request-id", reqID).
			WithField("response", rawResp).
			Error("Video source look up failed")
		return common.VideoSource{}, err
	}

	if len(videoSourceList.Sources) == 0 {
		err := fmt.Errorf("video source '%s' is unknown by controller", sourceName)
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source", sourceName).
			WithField("outbound-request-id", reqID).
			Error("Video source look up failed")
		return common.VideoSource{}, err
	}

	return videoSourceList.Sources[0], nil
}

func (c *restControlRequestClientImpl) ListActiveRecordingsOfSource(
	ctxt context.Context, sourceID string,
) ([]common.Recording, error) {
	logTags := c.GetLogTagsForContext(ctxt)

	reqID := ulid.Make().String()

	requestURL := c.ctrlBaseURI.JoinPath(fmt.Sprintf("/v1/source/%s/recording", sourceID))
	resp, err := c.client.R().
		// Set request header for segment reception
		SetHeader(c.requestIDHeader, reqID).
		// Set query parameters
		SetQueryParam("only_active", "true").
		// Set response payload
		SetResult(&common.RecordingSessionListResponse{}).
		// Setup error parsing
		SetError(goutils.RestAPIBaseResponse{}).
		Get(requestURL.String())

	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			Error("Active recordings look up request failed on call")
		return nil, err
	}

	if !resp.IsSuccess() {
		respError := resp.Error().(*goutils.RestAPIBaseResponse)
		var err error
		if respError.Error != nil {
			err = fmt.Errorf("%s", respError.Error.Detail)
		} else {
			err = fmt.Errorf("status code %d", resp.StatusCode())
		}
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("outbound-request-id", reqID).
			Error("Active recordings look up failed")
		return nil, err
	}

	// Process the response
	recordingList, ok := resp.Result().(*common.RecordingSessionListResponse)
	if !ok {
		rawResp := string(resp.Body())
		err := fmt.Errorf("failed to parse list recording response")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("outbound-request-id", reqID).
			WithField("response", rawResp).
			Error("Active recordings look up failed")
		return nil, err
	}

	return recordingList.Recordings, nil
}

func (c *restControlRequestClientImpl) ExchangeVideoSourceStatus(
	ctxt context.Context, sourceID string, reqRespTargetID string, localTime time.Time,
) (common.VideoSource, []common.Recording, error) {
	logTags := c.GetLogTagsForContext(ctxt)

	reqID := ulid.Make().String()

	requestPayload := common.VideoSourceStatusReport{
		RequestResponseTargetID: reqRespTargetID, LocalTimestamp: localTime,
	}
	requestURL := c.ctrlBaseURI.JoinPath(fmt.Sprintf("/v1/source/%s/status", sourceID))
	resp, err := c.client.R().
		// Set request header for segment reception
		SetHeader(c.requestIDHeader, reqID).
		// Set request payload
		SetBody(requestPayload).
		// Set response payload
		SetResult(&common.VideoSourceCurrentStateResponse{}).
		// Setup error parsing
		SetError(goutils.RestAPIBaseResponse{}).
		Post(requestURL.String())

	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			Error("Video source status report failed on call")
		return common.VideoSource{}, nil, err
	}

	if !resp.IsSuccess() {
		respError := resp.Error().(*goutils.RestAPIBaseResponse)
		var err error
		if respError.Error != nil {
			err = fmt.Errorf("%s", respError.Error.Detail)
		} else {
			err = fmt.Errorf("status code %d", resp.StatusCode())
		}
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("outbound-request-id", reqID).
			Error("Video source status report failed")
		return common.VideoSource{}, nil, err
	}

	// Process the response
	statusReport, ok := resp.Result().(*common.VideoSourceCurrentStateResponse)
	if !ok {
		rawResp := string(resp.Body())
		err := fmt.Errorf("failed to parse video status report response")
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("outbound-request-id", reqID).
			WithField("response", rawResp).
			Error("Video source status report failed")
		return common.VideoSource{}, nil, err
	}

	return statusReport.Source, statusReport.Recordings, nil
}

func (c *restControlRequestClientImpl) StopAllAssociatedRecordings(
	_ context.Context, _ string,
) error {
	return nil
}
