package api_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestVodLiveStreamHandler(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockDB := mocks.NewPersistenceManager(t)
	mockPlaylist := mocks.NewPlaylistBuilder(t)
	mockSegment := mocks.NewSegmentManager(t)

	uut, err := api.NewLiveStreamHandler(
		mockDB, mockPlaylist, mockSegment, common.HTTPRequestLogging{
			RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
		},
	)
	assert.Nil(err)

	// Case 0: unknown video source
	{
		// Prepare request
		sourceID := uuid.NewString()
		target := "vid.m3u8"
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/live/%s/%s", sourceID, target), nil,
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/live/{videoSourceID}/{fileName}", uut.LoggingMiddleware(uut.GetVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.valueCtx"),
			sourceID,
		).Return(common.VideoSource{}, fmt.Errorf("dummy error")).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 1: known video source, failed to get playlist
	{
		// Prepare request
		sourceID := uuid.NewString()
		target := "vid.m3u8"
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/live/%s/%s", sourceID, target), nil,
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/live/{videoSourceID}/{fileName}", uut.LoggingMiddleware(uut.GetVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.valueCtx"),
			sourceID,
		).Return(common.VideoSource{ID: sourceID}, nil).Once()
		mockPlaylist.On(
			"GetLiveStreamPlaylist",
			mock.AnythingOfType("*context.valueCtx"),
			common.VideoSource{ID: sourceID},
			mock.AnythingOfType("time.Time"),
		).Return(hls.Playlist{}, fmt.Errorf("dummy error")).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusInternalServerError, respRecorder.Code)
	}

	// Case 2: known video source, got valid playlist
	{
		// Prepare request
		sourceID := uuid.NewString()
		target := "vid.m3u8"
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/live/%s/%s", sourceID, target), nil,
		)
		assert.Nil(err)

		testPlaylist := hls.Playlist{
			Name:              target,
			CreatedAt:         time.Now().UTC(),
			Version:           3,
			TargetSegDuration: 5.0,
			Segments:          []hls.Segment{{Name: "segment-0.ts", Length: 4.0}},
		}

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/live/{videoSourceID}/{fileName}", uut.LoggingMiddleware(uut.GetVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetVideoSource", mock.AnythingOfType("*context.valueCtx"), sourceID,
		).Return(common.VideoSource{ID: sourceID}, nil).Once()
		mockPlaylist.On(
			"GetLiveStreamPlaylist",
			mock.AnythingOfType("*context.valueCtx"),
			common.VideoSource{ID: sourceID},
			mock.AnythingOfType("time.Time"),
		).Return(testPlaylist, nil).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		t, err := testPlaylist.String()
		assert.Nil(err)
		resp := respRecorder.Body.String()
		{
			// The response comes with an extra "null" at the end
			resp = strings.ReplaceAll(resp, "null", "")
		}
		assert.Equal(t, resp)
	}

	// Case 3: known video source, unknown segment
	{
		// Prepare request
		sourceID := uuid.NewString()
		target := "vid-00.ts"
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/live/%s/%s", sourceID, target), nil,
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/live/{videoSourceID}/{fileName}", uut.LoggingMiddleware(uut.GetVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetVideoSource", mock.AnythingOfType("*context.valueCtx"), sourceID,
		).Return(common.VideoSource{ID: sourceID}, nil).Once()
		mockDB.On(
			"GetLiveStreamSegmentByName", mock.AnythingOfType("*context.valueCtx"), target,
		).Return(common.VideoSegment{}, fmt.Errorf("dummy error")).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 4: known video source, valid segment
	{
		// Prepare request
		sourceID := uuid.NewString()
		target := "vid-00.ts"
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/live/%s/%s", sourceID, target), nil,
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/live/{videoSourceID}/{fileName}", uut.LoggingMiddleware(uut.GetVideoFilesHandler()),
		)

		testSegment := common.VideoSegment{Segment: hls.Segment{Name: target}}
		testSegContent := []byte(uuid.NewString())

		// Prepare mock
		mockDB.On(
			"GetVideoSource", mock.AnythingOfType("*context.valueCtx"), sourceID,
		).Return(common.VideoSource{ID: sourceID}, nil).Once()
		mockDB.On(
			"GetLiveStreamSegmentByName", mock.AnythingOfType("*context.valueCtx"), target,
		).Return(testSegment, nil).Once()
		mockSegment.On(
			"GetSegment", mock.AnythingOfType("*context.valueCtx"), testSegment,
		).Return(testSegContent, nil).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		resp, err := io.ReadAll(respRecorder.Result().Body)
		assert.Nil(err)
		log.Debugf("Response length %d", len(resp))
		{
			// The response comes with an extra "null" at the end
			resp = resp[0 : len(resp)-4]
		}
		assert.Equal(testSegContent, resp)
	}
}
