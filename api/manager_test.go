package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestManagerDefineNewVideoSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	// Case 0: no parameters given
	{
		req, err := http.NewRequest("POST", "/v1/source", nil)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source", uut.LoggingMiddleware(uut.DefineNewVideoSourceHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 1: non-json payload
	{
		payload := uuid.NewString()
		req, err := http.NewRequest("POST", "/v1/source", bytes.NewBufferString(payload))
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source", uut.LoggingMiddleware(uut.DefineNewVideoSourceHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 2: parameter has wrong format
	{
		payload := api.NewVideoSourceRequest{
			Name: uuid.NewString(),
			PlaylistURI: func() *string {
				t := "hello world"
				return &t
			}(),
		}
		payloadByte, err := json.Marshal(&payload)
		assert.Nil(err)
		req, err := http.NewRequest("POST", "/v1/source", bytes.NewBuffer(payloadByte))
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source", uut.LoggingMiddleware(uut.DefineNewVideoSourceHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 3: correct parameters
	{
		payload := api.NewVideoSourceRequest{Name: uuid.NewString(), TargetSegmentLen: 2}
		payloadByte, err := json.Marshal(&payload)
		assert.Nil(err)
		req, err := http.NewRequest("POST", "/v1/source", bytes.NewBuffer(payloadByte))
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source", uut.LoggingMiddleware(uut.DefineNewVideoSourceHandler()),
		)

		// Prepare mock
		testEntryID := uuid.NewString()
		testSource := common.VideoSource{ID: testEntryID, Name: payload.Name}
		mockManager.On(
			"DefineVideoSource",
			mock.AnythingOfType("*context.valueCtx"),
			payload.Name,
			2,
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
		).Return(testEntryID, nil).Once()
		mockManager.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.valueCtx"),
			testEntryID,
		).Return(testSource, nil).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		// Verify response
		var resp api.VideoSourceInfoResponse
		assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
		assert.EqualValues(testSource, resp.Source)
	}
}

func TestManagerListVideoSources(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testSources := []common.VideoSource{}
	for itr := 0; itr < 3; itr++ {
		testSources = append(testSources, common.VideoSource{
			ID: uuid.NewString(), Name: uuid.NewString(),
		})
	}

	// Setup mock
	mockManager.On(
		"ListVideoSources",
		mock.AnythingOfType("*context.valueCtx"),
	).Return(testSources, nil).Once()

	// Prepare request
	req, err := http.NewRequest("GET", "/v1/source", nil)
	assert.Nil(err)

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/source", uut.LoggingMiddleware(uut.ListVideoSourcesHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
	// Verify response
	var resp common.VideoSourceInfoListResponse
	assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
	assert.EqualValues(testSources, resp.Sources)
}

func TestManagerListOneTargetVideoSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testSource := common.VideoSource{ID: uuid.NewString(), Name: uuid.NewString()}

	// Setup mock
	mockManager.On(
		"GetVideoSourceByName",
		mock.AnythingOfType("*context.valueCtx"),
		testSource.Name,
	).Return(testSource, nil).Once()

	// Prepare request
	req, err := http.NewRequest("GET", "/v1/source", nil)
	assert.Nil(err)
	{
		q := req.URL.Query()
		q.Add("source_name", testSource.Name)
		req.URL.RawQuery = q.Encode()
	}

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/source", uut.LoggingMiddleware(uut.ListVideoSourcesHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
	// Verify response
	var resp common.VideoSourceInfoListResponse
	assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
	assert.Len(resp.Sources, 1)
	assert.EqualValues(testSource, resp.Sources[0])
}

func TestManagerGetVideoSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	// Prepare mock
	testEntryID := uuid.NewString()
	testSource := common.VideoSource{ID: testEntryID, Name: uuid.NewString()}
	mockManager.On(
		"GetVideoSource",
		mock.AnythingOfType("*context.valueCtx"),
		testEntryID,
	).Return(testSource, nil).Once()

	// Prepare request
	req, err := http.NewRequest("GET", fmt.Sprintf("/v1/source/%s", testEntryID), nil)
	assert.Nil(err)

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/source/{sourceID}", uut.LoggingMiddleware(uut.GetVideoSourceHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
	// Verify response
	var resp api.VideoSourceInfoResponse
	assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
	assert.EqualValues(testSource, resp.Source)
}

func TestManagerDeleteVideoSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	// Prepare mock
	testEntryID := uuid.NewString()
	mockManager.On(
		"DeleteVideoSource",
		mock.AnythingOfType("*context.valueCtx"),
		testEntryID,
	).Return(nil).Once()

	// Prepare request
	req, err := http.NewRequest("DELETE", fmt.Sprintf("/v1/source/%s", testEntryID), nil)
	assert.Nil(err)

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/source/{sourceID}", uut.LoggingMiddleware(uut.DeleteVideoSourceHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
}

func TestManagerUpdateVideoSourceName(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	// Prepare mock
	testEntryID := uuid.NewString()
	testSource := common.VideoSource{ID: testEntryID, Name: uuid.NewString()}
	newName := uuid.NewString()
	mockManager.On(
		"GetVideoSource",
		mock.AnythingOfType("*context.valueCtx"),
		testEntryID,
	).Return(testSource, nil).Once()
	testSourceNew := common.VideoSource{ID: testEntryID, Name: newName}
	mockManager.On(
		"UpdateVideoSource",
		mock.AnythingOfType("*context.valueCtx"),
		testSourceNew,
	).Return(nil).Once()

	// Prepare request
	req, err := http.NewRequest("PUT", fmt.Sprintf("/v1/source/%s", testEntryID), nil)
	assert.Nil(err)
	{
		q := req.URL.Query()
		q.Add("new_name", newName)
		req.URL.RawQuery = q.Encode()
	}

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/source/{sourceID}", uut.LoggingMiddleware(uut.UpdateVideoSourceNameHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
}

func TestManagerChangeVideoStreamingState(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	// Prepare mock
	testEntryID := uuid.NewString()
	newState := "false"
	mockManager.On(
		"ChangeVideoSourceStreamState",
		mock.AnythingOfType("*context.valueCtx"),
		testEntryID,
		-1,
	).Return(nil).Once()

	// Prepare request
	req, err := http.NewRequest("PUT", fmt.Sprintf("/v1/source/%s/streaming", testEntryID), nil)
	assert.Nil(err)
	{
		q := req.URL.Query()
		q.Add("new_state", newState)
		req.URL.RawQuery = q.Encode()
	}

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/source/{sourceID}/streaming",
		uut.LoggingMiddleware(uut.ChangeSourceStreamingStateHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
}

func TestManagerExchangeVideoSourceStatusInfo(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testSource := common.VideoSource{ID: uuid.NewString()}

	// Case 0: no payload provided
	{
		req, err := http.NewRequest("POST", fmt.Sprintf("/v1/source/%s/status", testSource.ID), nil)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source/{sourceID}/status",
			uut.LoggingMiddleware(uut.ExchangeVideoSourceStatusInfoHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 1: non-json payload
	{
		payload := uuid.NewString()
		req, err := http.NewRequest(
			"POST", fmt.Sprintf("/v1/source/%s/status", testSource.ID), bytes.NewBufferString(payload),
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source/{sourceID}/status",
			uut.LoggingMiddleware(uut.ExchangeVideoSourceStatusInfoHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 2: incorrect format
	{
		payload := common.VideoSourceStatusReport{}
		payloadByte, err := json.Marshal(&payload)
		assert.Nil(err)
		req, err := http.NewRequest(
			"POST", fmt.Sprintf("/v1/source/%s/status", testSource.ID), bytes.NewBuffer(payloadByte),
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source/{sourceID}/status",
			uut.LoggingMiddleware(uut.ExchangeVideoSourceStatusInfoHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 3: correct format
	{
		testRecordings := []common.Recording{{ID: uuid.NewString()}, {ID: uuid.NewString()}}
		testReqRespTargetID := uuid.NewString()
		currentTime := time.Now().UTC()

		// Prepare mock
		mockManager.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.valueCtx"),
			testSource.ID,
		).Return(testSource, nil).Once()
		mockManager.On(
			"UpdateVideoSourceStatus",
			mock.AnythingOfType("*context.valueCtx"),
			testSource.ID,
			testReqRespTargetID,
			currentTime,
		).Return(nil).Once()
		mockManager.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.valueCtx"),
			testSource.ID,
			true,
		).Return(testRecordings, nil).Once()

		payload := common.VideoSourceStatusReport{
			RequestResponseTargetID: testReqRespTargetID, LocalTimestamp: currentTime,
		}
		payloadByte, err := json.Marshal(&payload)
		assert.Nil(err)
		req, err := http.NewRequest(
			"POST", fmt.Sprintf("/v1/source/%s/status", testSource.ID), bytes.NewBuffer(payloadByte),
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source/{sourceID}/status",
			uut.LoggingMiddleware(uut.ExchangeVideoSourceStatusInfoHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		// Verify response
		var resp common.VideoSourceCurrentStateResponse
		assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
		assert.EqualValues(testSource, resp.Source)
		assert.EqualValues(testRecordings, resp.Recordings)
	}
}

func TestManagerStartRecordingSession(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testSourceID := uuid.NewString()

	// Case 0: no parameters given
	{
		req, err := http.NewRequest(
			"POST", fmt.Sprintf("/v1/source/%s/recording", testSourceID), bytes.NewBuffer([]byte{}),
		)
		assert.Nil(err)

		// Prepare mock
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSourceID}
		mockManager.On(
			"DefineRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			testSourceID,
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("time.Time"),
		).Return(testRecording.ID, nil).Once()
		mockManager.On(
			"GetRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source/{sourceID}/recording", uut.LoggingMiddleware(uut.StartNewRecordingHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		// Verify response
		var resp api.RecordingSessionResponse
		assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
		assert.Equal(testRecording.ID, resp.Recording.ID)
		assert.Equal(testRecording.SourceID, resp.Recording.SourceID)
	}

	// Case 1: correct parameters
	{
		timestamp := time.Now().UTC()
		alias := uuid.NewString()
		description := uuid.NewString()
		payload := api.StartNewRecordingRequest{
			Alias: &alias, Description: &description, StartTime: timestamp.Unix(),
		}
		payloadByte, err := json.Marshal(&payload)
		assert.Nil(err)
		req, err := http.NewRequest(
			"POST", fmt.Sprintf("/v1/source/%s/recording", testSourceID), bytes.NewBuffer(payloadByte),
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source/{sourceID}/recording", uut.LoggingMiddleware(uut.StartNewRecordingHandler()),
		)

		// Prepare mock
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: testSourceID}
		mockManager.On(
			"DefineRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			testSourceID,
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("*string"),
			mock.AnythingOfType("time.Time"),
		).Run(func(args mock.Arguments) {
			reqAlias, ok := args.Get(2).(*string)
			assert.True(ok)
			reqDesp, ok := args.Get(3).(*string)
			assert.True(ok)
			reqTime, ok := args.Get(4).(time.Time)
			assert.True(ok)

			assert.Equal(alias, *reqAlias)
			assert.Equal(description, *reqDesp)
			assert.Equal(timestamp.Unix(), reqTime.Unix())
		}).Return(testRecording.ID, nil).Once()
		mockManager.On(
			"GetRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		// Verify response
		var resp api.RecordingSessionResponse
		assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
		assert.Equal(testRecording.ID, resp.Recording.ID)
		assert.Equal(testRecording.SourceID, resp.Recording.SourceID)
	}
}

func TestManagerListRecordingSession(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	// Prepare mock
	testRecordings := []common.Recording{
		{ID: uuid.NewString(), SourceID: uuid.NewString()},
		{ID: uuid.NewString(), SourceID: uuid.NewString()},
		{ID: uuid.NewString(), SourceID: uuid.NewString()},
	}
	mockManager.On(
		"ListRecordingSessions",
		mock.AnythingOfType("*context.valueCtx"),
	).Return(testRecordings, nil).Once()

	req, err := http.NewRequest("GET", "/v1/recording", nil)
	assert.Nil(err)

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/recording", uut.LoggingMiddleware(uut.ListRecordingsHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
	// Verify response
	var resp common.RecordingSessionListResponse
	assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
	assert.Len(resp.Recordings, len(testRecordings))
	for idx, recording := range resp.Recordings {
		testRecording := testRecordings[idx]
		assert.Equal(testRecording.ID, recording.ID)
		assert.Equal(testRecording.SourceID, recording.SourceID)
	}
}

func TestManagerListRecordingOfSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testSourceID := uuid.NewString()

	testRecordings := []common.Recording{
		{ID: uuid.NewString(), SourceID: uuid.NewString()},
		{ID: uuid.NewString(), SourceID: uuid.NewString()},
		{ID: uuid.NewString(), SourceID: uuid.NewString()},
	}

	// Case 0: active only recording
	{
		req, err := http.NewRequest("POST", fmt.Sprintf("/v1/source/%s/recording", testSourceID), nil)
		assert.Nil(err)
		{
			q := req.URL.Query()
			q.Add("only_active", "true")
			req.URL.RawQuery = q.Encode()
		}

		// Prepare mock
		mockManager.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.valueCtx"),
			testSourceID,
			true,
		).Return(testRecordings, nil).Once()

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source/{sourceID}/recording", uut.LoggingMiddleware(uut.ListRecordingsOfSourceHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		// Verify response
		var resp common.RecordingSessionListResponse
		assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
		assert.Len(resp.Recordings, len(testRecordings))
		for idx, recording := range resp.Recordings {
			testRecording := testRecordings[idx]
			assert.Equal(testRecording.ID, recording.ID)
			assert.Equal(testRecording.SourceID, recording.SourceID)
		}
	}

	// Case 1: all recording
	{
		req, err := http.NewRequest("POST", fmt.Sprintf("/v1/source/%s/recording", testSourceID), nil)
		assert.Nil(err)

		// Prepare mock
		mockManager.On(
			"ListRecordingSessionsOfSource",
			mock.AnythingOfType("*context.valueCtx"),
			testSourceID,
			false,
		).Return(testRecordings, nil).Once()

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/source/{sourceID}/recording", uut.LoggingMiddleware(uut.ListRecordingsOfSourceHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		// Verify response
		var resp common.RecordingSessionListResponse
		assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
		assert.Len(resp.Recordings, len(testRecordings))
		for idx, recording := range resp.Recordings {
			testRecording := testRecordings[idx]
			assert.Equal(testRecording.ID, recording.ID)
			assert.Equal(testRecording.SourceID, recording.SourceID)
		}
	}
}

func TestManagerGetRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testRecording := common.Recording{ID: uuid.NewString(), SourceID: uuid.NewString()}

	// Prepare mock
	mockManager.On(
		"GetRecordingSession",
		mock.AnythingOfType("*context.valueCtx"),
		testRecording.ID,
	).Return(testRecording, nil).Once()

	req, err := http.NewRequest("GET", fmt.Sprintf("/v1/recording/%s", testRecording.ID), nil)
	assert.Nil(err)

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/recording/{recordingID}", uut.LoggingMiddleware(uut.GetRecordingHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
	// Verify response
	var resp api.RecordingSessionResponse
	assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
	assert.Equal(testRecording.ID, resp.Recording.ID)
	assert.Equal(testRecording.SourceID, resp.Recording.SourceID)
}

func TestManagerListSegmentsOfRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testRecordingID := uuid.NewString()

	testSegments := []common.VideoSegment{
		{
			ID:       uuid.NewString(),
			SourceID: uuid.NewString(),
			Segment:  hls.Segment{Name: uuid.NewString()},
		},
	}

	// Prepare mock
	mockManager.On(
		"ListAllSegmentsOfRecording",
		mock.AnythingOfType("*context.valueCtx"),
		testRecordingID,
	).Return(testSegments, nil).Once()

	req, err := http.NewRequest("GET", fmt.Sprintf("/v1/recording/%s/segment", testRecordingID), nil)
	assert.Nil(err)

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/recording/{recordingID}/segment",
		uut.LoggingMiddleware(uut.ListSegmentsOfRecordingHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
	// Verify response
	var resp api.RecordingSegmentListResponse
	assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
	assert.Len(resp.Segments, len(testSegments))
	for idx, gotSegment := range resp.Segments {
		testSegment := testSegments[idx]
		assert.Equal(testSegment.ID, gotSegment.ID)
		assert.Equal(testSegment.SourceID, gotSegment.SourceID)
		assert.Equal(testSegment.Name, gotSegment.Name)
	}
}

func TestManagerStopRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testRecording := common.Recording{ID: uuid.NewString(), SourceID: uuid.NewString()}

	// Prepare mock
	mockManager.On(
		"MarkEndOfRecordingSession",
		mock.AnythingOfType("*context.valueCtx"),
		testRecording.ID,
		mock.AnythingOfType("time.Time"),
		true,
	).Return(nil).Once()

	req, err := http.NewRequest("POST", fmt.Sprintf("/v1/recording/%s/stop", testRecording.ID), nil)
	assert.Nil(err)
	{
		q := req.URL.Query()
		q.Add("force", "true")
		req.URL.RawQuery = q.Encode()
	}

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/recording/{recordingID}/stop", uut.LoggingMiddleware(uut.StopRecordingHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
}

func TestManagerDeleteRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	}, nil)
	assert.Nil(err)

	testRecordingID := uuid.NewString()

	// Prepare mock
	mockManager.On(
		"DeleteRecordingSession",
		mock.AnythingOfType("*context.valueCtx"),
		testRecordingID,
		true,
	).Return(nil).Once()

	req, err := http.NewRequest("POST", fmt.Sprintf("/v1/recording/%s", testRecordingID), nil)
	assert.Nil(err)
	{
		q := req.URL.Query()
		q.Add("force", "true")
		req.URL.RawQuery = q.Encode()
	}

	// Setup HTTP handling
	router := mux.NewRouter()
	respRecorder := httptest.NewRecorder()
	router.HandleFunc(
		"/v1/recording/{recordingID}", uut.LoggingMiddleware(uut.DeleteRecordingHandler()),
	)

	// Request
	router.ServeHTTP(respRecorder, req)

	assert.Equal(http.StatusOK, respRecorder.Code)
}
