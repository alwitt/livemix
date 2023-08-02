package api_test

import (
	"fmt"
	"io"
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

func TestVodLiveStreamHandler(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockPLManager := mocks.NewPlaylistManager(t)

	uut, err := api.NewVODHandler(
		mockSQL, mockPLManager, common.HTTPRequestLogging{
			RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
		}, nil,
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
			"/v1/vod/live/{videoSourceID}/{fileName}",
			uut.LoggingMiddleware(uut.GetLiveStreamVideoFilesHandler()),
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
			"/v1/vod/live/{videoSourceID}/{fileName}",
			uut.LoggingMiddleware(uut.GetLiveStreamVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.valueCtx"),
			sourceID,
		).Return(common.VideoSource{ID: sourceID}, nil).Once()
		mockPLManager.On(
			"GetLiveStreamPlaylist",
			mock.AnythingOfType("*context.valueCtx"),
			common.VideoSource{ID: sourceID},
			mock.AnythingOfType("time.Time"),
			true,
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
			"/v1/vod/live/{videoSourceID}/{fileName}",
			uut.LoggingMiddleware(uut.GetLiveStreamVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetVideoSource", mock.AnythingOfType("*context.valueCtx"), sourceID,
		).Return(common.VideoSource{ID: sourceID}, nil).Once()
		mockPLManager.On(
			"GetLiveStreamPlaylist",
			mock.AnythingOfType("*context.valueCtx"),
			common.VideoSource{ID: sourceID},
			mock.AnythingOfType("time.Time"),
			true,
		).Return(testPlaylist, nil).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		t, err := testPlaylist.String(true)
		assert.Nil(err)
		resp := respRecorder.Body.String()
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
			"/v1/vod/live/{videoSourceID}/{fileName}",
			uut.LoggingMiddleware(uut.GetLiveStreamVideoFilesHandler()),
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
			"/v1/vod/live/{videoSourceID}/{fileName}",
			uut.LoggingMiddleware(uut.GetLiveStreamVideoFilesHandler()),
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
		mockPLManager.On(
			"GetSegment", mock.AnythingOfType("*context.valueCtx"), testSegment,
		).Return(testSegContent, nil).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		resp, err := io.ReadAll(respRecorder.Result().Body)
		assert.Nil(err)
		log.Debugf("Response length %d", len(resp))
		assert.Equal(testSegContent, resp)
	}
}

func TestVodRecordingHandler(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockPLManager := mocks.NewPlaylistManager(t)

	uut, err := api.NewVODHandler(
		mockSQL, mockPLManager, common.HTTPRequestLogging{
			RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
		}, nil,
	)
	assert.Nil(err)

	// Case 0: unknown recording ID
	{
		recordingID := uuid.NewString()
		target := "vid.m3u8"
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/recording/%s/%s", recordingID, target), nil,
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/recording/{recordingID}/{fileName}",
			uut.LoggingMiddleware(uut.GetRecordingVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			recordingID,
		).Return(common.Recording{}, fmt.Errorf("dummy error")).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 1: known recording, failed to get playlist
	{
		target := "vid.m3u8"
		testRecording := common.Recording{ID: uuid.NewString()}

		// Prepare request
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/recording/%s/%s", testRecording.ID, target), nil,
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/recording/{recordingID}/{fileName}",
			uut.LoggingMiddleware(uut.GetRecordingVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockPLManager.On(
			"GetRecordingStreamPlaylist",
			mock.AnythingOfType("*context.valueCtx"),
			mock.AnythingOfType("common.Recording"),
		).Run(func(args mock.Arguments) {
			recording, ok := args.Get(1).(common.Recording)
			assert.True(ok)
			assert.Equal(testRecording.ID, recording.ID)
		}).Return(hls.Playlist{}, fmt.Errorf("dummy error")).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusInternalServerError, respRecorder.Code)
	}

	// Case 2: known recording, got valid playlist
	{
		target := "vid.m3u8"
		testRecording := common.Recording{ID: uuid.NewString()}

		// Prepare request
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/recording/%s/%s", testRecording.ID, target), nil,
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
			"/v1/vod/recording/{recordingID}/{fileName}",
			uut.LoggingMiddleware(uut.GetRecordingVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockPLManager.On(
			"GetRecordingStreamPlaylist",
			mock.AnythingOfType("*context.valueCtx"),
			mock.AnythingOfType("common.Recording"),
		).Run(func(args mock.Arguments) {
			recording, ok := args.Get(1).(common.Recording)
			assert.True(ok)
			assert.Equal(testRecording.ID, recording.ID)
		}).Return(testPlaylist, nil).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		t, err := testPlaylist.String(false)
		assert.Nil(err)
		resp := respRecorder.Body.String()
		assert.Equal(t, resp)
	}

	// Case 3: known recording, unknown segment
	{
		testRecording := common.Recording{ID: uuid.NewString()}
		target := "vid-00.ts"

		// Prepare request
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/recording/%s/%s", testRecording.ID, target), nil,
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/recording/{recordingID}/{fileName}",
			uut.LoggingMiddleware(uut.GetRecordingVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockDB.On(
			"GetRecordingSegmentByName",
			mock.AnythingOfType("*context.valueCtx"),
			target,
		).Return(common.VideoSegment{}, fmt.Errorf("dummy error")).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 4: known recording, valid segment
	{
		testRecording := common.Recording{ID: uuid.NewString()}
		target := "vid-00.ts"
		testSegment := common.VideoSegment{
			ID: uuid.NewString(), SourceID: uuid.NewString(), Segment: hls.Segment{Name: target},
		}
		testSegContent := []byte(uuid.NewString())

		// Prepare request
		req, err := http.NewRequest(
			"GET", fmt.Sprintf("/v1/vod/recording/%s/%s", testRecording.ID, target), nil,
		)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/vod/recording/{recordingID}/{fileName}",
			uut.LoggingMiddleware(uut.GetRecordingVideoFilesHandler()),
		)

		// Prepare mock
		mockDB.On(
			"GetRecordingSession",
			mock.AnythingOfType("*context.valueCtx"),
			testRecording.ID,
		).Return(testRecording, nil).Once()
		mockDB.On(
			"GetRecordingSegmentByName",
			mock.AnythingOfType("*context.valueCtx"),
			target,
		).Return(testSegment, nil).Once()
		mockPLManager.On(
			"GetSegment",
			mock.AnythingOfType("*context.valueCtx"),
			testSegment,
		).Return(testSegContent, nil).Once()

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		resp, err := io.ReadAll(respRecorder.Result().Body)
		assert.Nil(err)
		log.Debugf("Response length %d", len(resp))
		assert.Equal(testSegContent, resp)
	}
}
