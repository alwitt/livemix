package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
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
	})
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
		payload := api.NewVideoSourceRequest{Name: uuid.NewString()}
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
	})
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
	var resp api.VideoSourceInfoListResponse
	assert.Nil(json.Unmarshal(respRecorder.Body.Bytes(), &resp))
	assert.EqualValues(testSources, resp.Sources)
}

func TestManagerGetVideoSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockManager := mocks.NewSystemManager(t)

	uut, err := api.NewSystemManagerHandler(mockManager, common.HTTPRequestLogging{
		RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
	})
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
	})
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
	})
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
