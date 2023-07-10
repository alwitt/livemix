package api_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alwitt/livemix/api"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
)

func TestPlaylistReceiver(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	parser := hls.NewPlaylistParser()

	parsedPlaylist := hls.Playlist{}

	receivePlaylist := func(ctxt context.Context, playlist hls.Playlist, ts time.Time) error {
		parsedPlaylist = playlist
		return nil
	}

	uut, err := api.NewPlaylistReceiveHandler(
		utCtxt, parser, receivePlaylist, common.HTTPRequestLogging{
			RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
		},
	)
	assert.Nil(err)

	// Case 0: missing required headers
	{
		req, err := http.NewRequest("POST", "/v1/playlist", nil)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/playlist", uut.LoggingMiddleware(uut.NewPlaylistHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 1: correct request
	{
		payload := strings.Join(
			[]string{
				"#EXTM3U",
				"#EXT-X-VERSION:3",
				"#EXT-X-TARGETDURATION:62",
				"#EXT-X-MEDIA-SEQUENCE:0",
				"#EXTINF:62.500000,",
				"vid-0.ts",
				"#EXTINF:23.500000,",
				"vid-1.ts",
				"#EXT-X-ENDLIST",
			},
			"\n",
		)
		req, err := http.NewRequest("POST", "/v1/playlist", bytes.NewBufferString(payload))
		assert.Nil(err)
		req.Header.Add("Video-Source-Name", "testing")
		req.Header.Add("MPEG-TS-URI-Prefix", "file:///vid")

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/playlist", uut.LoggingMiddleware(uut.NewPlaylistHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		assert.Equal("testing", parsedPlaylist.Name)
		assert.Equal(62.0, parsedPlaylist.TargetSegDuration)
		assert.Equal(3, parsedPlaylist.Version)
		assert.Len(parsedPlaylist.Segments, 2)
		assert.Equal("vid-0.ts", parsedPlaylist.Segments[0].Name)
		assert.Equal("file:///vid/vid-0.ts", parsedPlaylist.Segments[0].URI)
		assert.Equal(62.5, parsedPlaylist.Segments[0].Length)
		assert.Equal("vid-1.ts", parsedPlaylist.Segments[1].Name)
		assert.Equal("file:///vid/vid-1.ts", parsedPlaylist.Segments[1].URI)
		assert.Equal(23.5, parsedPlaylist.Segments[1].Length)
	}
}

func TestSegmentReceive(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	rxSourceID := ""
	rxSegmentInfo := hls.Segment{}
	rxContent := []byte{}

	receiveSegment := func(
		ctxt context.Context, sourceID string, segment hls.Segment, content []byte,
	) error {
		rxSourceID = sourceID
		rxSegmentInfo = segment
		rxContent = content
		return nil
	}

	uut, err := api.NewSegmentReceiveHandler(
		utCtxt, receiveSegment, common.HTTPRequestLogging{
			RequestIDHeader: "X-Request-ID", DoNotLogHeaders: []string{},
		},
	)
	assert.Nil(err)

	// Case 0: missing required header
	{
		req, err := http.NewRequest("POST", "/v1/new-segment", nil)
		assert.Nil(err)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/new-segment", uut.LoggingMiddleware(uut.NewSegmentHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusBadRequest, respRecorder.Code)
	}

	// Case 1: correct request
	{
		testSourceID := uuid.NewString()
		testSegmentName := uuid.NewString()
		testStartTime := time.Now().UTC()
		testSegmentLen := time.Second * 4
		testURI := fmt.Sprintf("file:///tmp/%s", testSegmentName)
		payload := []byte(uuid.NewString())
		req, err := http.NewRequest("POST", "/v1/new-segment", bytes.NewBuffer(payload))
		assert.Nil(err)
		req.Header.Add(ipc.HTTPSegmentForwardHeaderSourceID, testSourceID)
		req.Header.Add(ipc.HTTPSegmentForwardHeaderName, testSegmentName)
		req.Header.Add(ipc.HTTPSegmentForwardHeaderStartTS, fmt.Sprintf("%d", testStartTime.Unix()))
		req.Header.Add(
			ipc.HTTPSegmentForwardHeaderLength, fmt.Sprintf("%d", int(testSegmentLen.Milliseconds())),
		)
		req.Header.Add(ipc.HTTPSegmentForwardHeaderSegURI, testURI)

		// Setup HTTP handling
		router := mux.NewRouter()
		respRecorder := httptest.NewRecorder()
		router.HandleFunc(
			"/v1/new-segment", uut.LoggingMiddleware(uut.NewSegmentHandler()),
		)

		// Request
		router.ServeHTTP(respRecorder, req)

		assert.Equal(http.StatusOK, respRecorder.Code)
		assert.Equal(testSourceID, rxSourceID)
		assert.Equal(payload, rxContent)
		assert.Equal(testSegmentName, rxSegmentInfo.Name)
		assert.Equal(testStartTime.Unix(), rxSegmentInfo.StartTime.Unix())
		assert.Equal(testStartTime.Add(testSegmentLen).Unix(), rxSegmentInfo.EndTime.Unix())
		assert.Equal(testSegmentLen.Seconds(), rxSegmentInfo.Length)
		assert.Equal(testURI, rxSegmentInfo.URI)
	}
}
