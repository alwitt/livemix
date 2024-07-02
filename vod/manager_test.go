package vod_test

import (
	"context"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/alwitt/livemix/vod"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestVideoManagerPrefetch(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockBuilder := mocks.NewPlaylistBuilder(t)
	mockSegMgmt := mocks.NewSegmentManager(t)

	uut, err := vod.NewPlaylistManager(utCtxt, mockSQL, 4, mockBuilder, mockSegMgmt, nil)
	assert.Nil(err)

	// ------------------------------------------------------------------------------------
	// Case 0: playlist contains only one segment

	{
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: uuid.NewString()}
		testPlaylist := hls.Playlist{Segments: []hls.Segment{{Name: uuid.NewString()}}}

		// Prepare mock
		mockBuilder.On(
			"GetRecordingStreamPlaylist",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording,
		).Return(testPlaylist, nil).Once()

		builtPlaylist, err := uut.GetRecordingStreamPlaylist(utCtxt, testRecording)
		assert.Nil(err)
		assert.EqualValues(testPlaylist, builtPlaylist)
	}

	// ------------------------------------------------------------------------------------
	// Case 1: playlist contains three segment

	{
		testRecording := common.Recording{ID: uuid.NewString(), SourceID: uuid.NewString()}
		testSegments := []common.VideoSegment{
			{
				ID:       uuid.NewString(),
				SourceID: testRecording.SourceID,
				Segment:  hls.Segment{Name: uuid.NewString()},
			},
			{
				ID:       uuid.NewString(),
				SourceID: testRecording.SourceID,
				Segment:  hls.Segment{Name: uuid.NewString()},
			},
			{
				ID:       uuid.NewString(),
				SourceID: testRecording.SourceID,
				Segment:  hls.Segment{Name: uuid.NewString()},
			},
		}
		testPlaylist := hls.Playlist{Segments: []hls.Segment{}}
		for _, oneSeg := range testSegments {
			testPlaylist.Segments = append(testPlaylist.Segments, oneSeg.Segment)
		}

		prefetchCall := make(chan bool, 4)

		// Prepare mock
		mockBuilder.On(
			"GetRecordingStreamPlaylist",
			mock.AnythingOfType("context.backgroundCtx"),
			testRecording,
		).Return(testPlaylist, nil).Once()
		for idx, oneSeg := range testSegments {
			if idx == 0 {
				continue
			}
			mockDB.On(
				"GetRecordingSegmentByName",
				mock.AnythingOfType("context.backgroundCtx"),
				oneSeg.Name,
			).Return(oneSeg, nil).Once()
			mockSegMgmt.On(
				"GetSegment",
				mock.AnythingOfType("*context.cancelCtx"),
				oneSeg,
			).Run(func(args mock.Arguments) {
				prefetchCall <- true
			}).Return([]byte("hello world"), nil).Once()
		}

		builtPlaylist, err := uut.GetRecordingStreamPlaylist(utCtxt, testRecording)
		assert.Nil(err)
		assert.EqualValues(testPlaylist, builtPlaylist)

		lclCtxt, lclCtxtCancel := context.WithTimeout(utCtxt, time.Second)
		defer lclCtxtCancel()
		// Wait for prefetch calls to occur
		for itr := 0; itr < 2; itr++ {
			select {
			case <-lclCtxt.Done():
				assert.False(true, "timed out waiting for prefetch calls")
			case <-prefetchCall:
				continue
			}
		}
	}

	// Clean up
	assert.Nil(uut.Stop(utCtxt))
}
