package edge_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/edge"
	"github.com/apex/log"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/jarcoal/httpmock"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
)

func TestEdgeToControlRESTGetVideoSourceInfoRequest(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	testClient := resty.New()
	// Install with mock
	httpmock.ActivateNonDefault(testClient.GetClient())

	testBaseURL, err := url.Parse("http://ut.testing.dev/ctrl")
	assert.Nil(err)

	uut, err := edge.NewRestControlRequestClient(utCtxt, testBaseURL, "request-id", testClient)
	assert.Nil(err)

	// Case 0: known video source
	{
		httpmock.Reset()
		testSource := common.VideoSource{
			ID:        uuid.NewString(),
			Name:      fmt.Sprintf("video %s", uuid.NewString()),
			Streaming: 1,
		}

		// Prepare mock
		testURL := testBaseURL.JoinPath("/v1/source")
		httpmock.RegisterResponder(
			"GET",
			testURL.String(),
			func(r *http.Request) (*http.Response, error) {
				// Verify the headers
				assert.NotEmpty(r.Header.Get("request-id"))
				// Verify query parameter
				queries := r.URL.Query()
				sourceName := queries.Get("source_name")
				assert.Equal(testSource.Name, sourceName)

				// Send response
				return httpmock.NewJsonResponse(http.StatusOK, common.VideoSourceInfoListResponse{
					RestAPIBaseResponse: goutils.RestAPIBaseResponse{Success: true},
					Sources:             []common.VideoSource{testSource},
				})
			},
		)

		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		source, err := uut.GetVideoSourceInfo(lclCtxt, testSource.Name)
		assert.Nil(err)
		assert.Equal(testSource.ID, source.ID)
		assert.Equal(testSource.Streaming, source.Streaming)
		lclCancel()
	}

	// Case 1: unknown video source
	{
		httpmock.Reset()
		testSourceName := uuid.NewString()

		// Prepare mock
		testURL := testBaseURL.JoinPath("/v1/source")
		httpmock.RegisterResponder(
			"GET",
			testURL.String(),
			func(r *http.Request) (*http.Response, error) {
				// Verify the headers
				requestID := r.Header.Get("request-id")
				assert.NotEmpty(requestID)
				// Verify query parameter
				queries := r.URL.Query()
				sourceName := queries.Get("source_name")
				assert.Equal(testSourceName, sourceName)

				// Send response
				return httpmock.NewJsonResponse(http.StatusOK, goutils.RestAPIBaseResponse{
					Success:   false,
					RequestID: requestID,
					Error: &goutils.ErrorDetail{
						Code: http.StatusBadRequest, Msg: "unknown", Detail: "video source is unknown",
					},
				})
			},
		)

		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		_, err := uut.GetVideoSourceInfo(lclCtxt, testSourceName)
		assert.NotNil(err)
		lclCancel()
	}
}

func TestEdgeToControlRESTListActiveRecordingsOfSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	testClient := resty.New()
	// Install with mock
	httpmock.ActivateNonDefault(testClient.GetClient())

	testBaseURL, err := url.Parse("http://ut.testing.dev/ctrl")
	assert.Nil(err)

	uut, err := edge.NewRestControlRequestClient(utCtxt, testBaseURL, "request-id", testClient)
	assert.Nil(err)

	// Case 0: unknown video ID
	{
		httpmock.Reset()
		testSource := uuid.NewString()

		// Prepare mock
		testURL := testBaseURL.JoinPath(fmt.Sprintf("/v1/source/%s/recording", testSource))
		httpmock.RegisterResponder(
			"GET",
			testURL.String(),
			func(r *http.Request) (*http.Response, error) {
				// Verify the headers
				requestID := r.Header.Get("request-id")
				assert.NotEmpty(requestID)
				// Verify query parameter
				queries := r.URL.Query()
				sourceName := queries.Get("only_active")
				assert.Equal("true", sourceName)

				// Send response
				return httpmock.NewJsonResponse(http.StatusNotFound, goutils.RestAPIBaseResponse{
					Success:   false,
					RequestID: requestID,
					Error: &goutils.ErrorDetail{
						Code: http.StatusBadRequest, Msg: "unknown", Detail: "video source is unknown",
					},
				})
			},
		)

		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		_, err := uut.ListActiveRecordingsOfSource(lclCtxt, testSource)
		assert.NotNil(err)
		lclCancel()
	}

	// Case 1: no active recordings
	{
		httpmock.Reset()
		testSource := uuid.NewString()

		// Prepare mock
		testURL := testBaseURL.JoinPath(fmt.Sprintf("/v1/source/%s/recording", testSource))
		httpmock.RegisterResponder(
			"GET",
			testURL.String(),
			func(r *http.Request) (*http.Response, error) {
				// Verify the headers
				requestID := r.Header.Get("request-id")
				assert.NotEmpty(requestID)
				// Verify query parameter
				queries := r.URL.Query()
				sourceName := queries.Get("only_active")
				assert.Equal("true", sourceName)

				// Send response
				return httpmock.NewJsonResponse(http.StatusOK, common.RecordingSessionListResponse{
					RestAPIBaseResponse: goutils.RestAPIBaseResponse{Success: true},
				})
			},
		)

		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		recordings, err := uut.ListActiveRecordingsOfSource(lclCtxt, testSource)
		assert.Nil(err)
		assert.Len(recordings, 0)
		lclCancel()
	}

	// Case 2: have active recordings
	{
		httpmock.Reset()
		testSource := uuid.NewString()
		testRecordings := []common.Recording{
			{ID: ulid.Make().String(), SourceID: testSource},
			{ID: ulid.Make().String(), SourceID: testSource},
			{ID: ulid.Make().String(), SourceID: testSource},
		}

		// Prepare mock
		testURL := testBaseURL.JoinPath(fmt.Sprintf("/v1/source/%s/recording", testSource))
		httpmock.RegisterResponder(
			"GET",
			testURL.String(),
			func(r *http.Request) (*http.Response, error) {
				// Verify the headers
				requestID := r.Header.Get("request-id")
				assert.NotEmpty(requestID)
				// Verify query parameter
				queries := r.URL.Query()
				sourceName := queries.Get("only_active")
				assert.Equal("true", sourceName)

				// Send response
				return httpmock.NewJsonResponse(http.StatusOK, common.RecordingSessionListResponse{
					RestAPIBaseResponse: goutils.RestAPIBaseResponse{Success: true},
					Recordings:          testRecordings,
				})
			},
		)

		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		recordings, err := uut.ListActiveRecordingsOfSource(lclCtxt, testSource)
		assert.Nil(err)
		assert.EqualValues(testRecordings, recordings)
		lclCancel()
	}
}

func TestEdgeToControlRESTExchangeVideoSourceStatus(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	testClient := resty.New()
	// Install with mock
	httpmock.ActivateNonDefault(testClient.GetClient())

	testBaseURL, err := url.Parse("http://ut.testing.dev/ctrl")
	assert.Nil(err)

	uut, err := edge.NewRestControlRequestClient(utCtxt, testBaseURL, "request-id", testClient)
	assert.Nil(err)

	currentTime := time.Now().UTC()

	// Case 0: unknown video ID
	{
		httpmock.Reset()
		testSource := uuid.NewString()

		// Prepare mock
		testURL := testBaseURL.JoinPath(fmt.Sprintf("/v1/source/%s/status", testSource))
		httpmock.RegisterResponder(
			"POST",
			testURL.String(),
			func(r *http.Request) (*http.Response, error) {
				// Verify the headers
				requestID := r.Header.Get("request-id")
				assert.NotEmpty(requestID)

				// Send response
				return httpmock.NewJsonResponse(http.StatusNotFound, goutils.RestAPIBaseResponse{
					Success:   false,
					RequestID: requestID,
					Error: &goutils.ErrorDetail{
						Code: http.StatusBadRequest, Msg: "unknown", Detail: "video source is unknown",
					},
				})
			},
		)

		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		_, _, err := uut.ExchangeVideoSourceStatus(lclCtxt, testSource, uuid.NewString(), currentTime)
		assert.NotNil(err)
		lclCancel()
	}

	// Case 1: no recordings
	{
		httpmock.Reset()
		testSource := common.VideoSource{
			ID:        uuid.NewString(),
			Name:      fmt.Sprintf("video %s", uuid.NewString()),
			Streaming: 1,
		}
		testReqRespTarget := uuid.NewString()

		// Prepare mock
		testURL := testBaseURL.JoinPath(fmt.Sprintf("/v1/source/%s/status", testSource.ID))
		httpmock.RegisterResponder(
			"POST",
			testURL.String(),
			func(r *http.Request) (*http.Response, error) {
				// Verify the headers
				requestID := r.Header.Get("request-id")
				assert.NotEmpty(requestID)
				// Verify the payload
				var params common.VideoSourceStatusReport
				assert.Nil(json.NewDecoder(r.Body).Decode(&params))
				assert.Equal(testReqRespTarget, params.RequestResponseTargetID)
				assert.Equal(currentTime, params.LocalTimestamp)

				// Send response
				return httpmock.NewJsonResponse(http.StatusOK, common.VideoSourceCurrentStateResponse{
					RestAPIBaseResponse: goutils.RestAPIBaseResponse{Success: true},
					Source:              testSource,
				})
			},
		)

		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		source, recordings, err := uut.ExchangeVideoSourceStatus(
			lclCtxt, testSource.ID, testReqRespTarget, currentTime,
		)
		assert.Nil(err)
		assert.Empty(recordings)
		assert.Equal(testSource.ID, source.ID)
		assert.Equal(testSource.Streaming, source.Streaming)
		lclCancel()
	}

	// Case 2: have recordings
	{
		httpmock.Reset()
		testSource := common.VideoSource{
			ID:        uuid.NewString(),
			Name:      fmt.Sprintf("video %s", uuid.NewString()),
			Streaming: 1,
		}
		testRecordings := []common.Recording{
			{ID: ulid.Make().String(), SourceID: testSource.ID},
			{ID: ulid.Make().String(), SourceID: testSource.ID},
			{ID: ulid.Make().String(), SourceID: testSource.ID},
		}
		testReqRespTarget := uuid.NewString()

		// Prepare mock
		testURL := testBaseURL.JoinPath(fmt.Sprintf("/v1/source/%s/status", testSource.ID))
		httpmock.RegisterResponder(
			"POST",
			testURL.String(),
			func(r *http.Request) (*http.Response, error) {
				// Verify the headers
				requestID := r.Header.Get("request-id")
				assert.NotEmpty(requestID)
				// Verify the payload
				var params common.VideoSourceStatusReport
				assert.Nil(json.NewDecoder(r.Body).Decode(&params))
				assert.Equal(testReqRespTarget, params.RequestResponseTargetID)
				assert.Equal(currentTime, params.LocalTimestamp)

				// Send response
				return httpmock.NewJsonResponse(http.StatusOK, common.VideoSourceCurrentStateResponse{
					RestAPIBaseResponse: goutils.RestAPIBaseResponse{Success: true},
					Source:              testSource,
					Recordings:          testRecordings,
				})
			},
		)

		lclCtxt, lclCancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		source, recordings, err := uut.ExchangeVideoSourceStatus(
			lclCtxt, testSource.ID, testReqRespTarget, currentTime,
		)
		assert.Nil(err)
		assert.Len(recordings, len(testRecordings))
		assert.Equal(testSource.ID, source.ID)
		assert.Equal(testSource.Streaming, source.Streaming)
		assert.EqualValues(testRecordings, recordings)
		lclCancel()
	}
}
