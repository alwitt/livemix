package api_test

import (
	"encoding/json"
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

func TestEdgeListVideoSources(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()

	uut, err := api.NewEdgeAPIHandler(mockSQL, common.HTTPRequestLogging{
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
	mockDB.On(
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
