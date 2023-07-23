package vod_test

import (
	"context"
	"fmt"
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

func TestBuildLiveStreamPlaylist(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()

	testSource := common.VideoSource{
		ID:                  uuid.NewString(),
		TargetSegmentLength: 5,
		Name:                fmt.Sprintf("vid-%s.m3u8", uuid.NewString()),
	}
	testSource.PlaylistURI = func() *string {
		t := fmt.Sprintf("file:///%s", testSource.Name)
		return &t
	}()

	segmentLength := time.Second * time.Duration(testSource.TargetSegmentLength)

	segmentPerPlaylist := 3

	uut, err := vod.NewPlaylistBuilder(mockSQL, segmentPerPlaylist)
	assert.Nil(err)

	startTime := time.Now().UTC()

	// Helper function to define a list of segments as returned by PersistenceManager
	defineSegmentList := func(
		startTime time.Time, segIDs map[string]string, segments []string,
	) []common.VideoSegment {
		result := []common.VideoSegment{}

		timestamp := startTime
		// Define segments
		for _, segmentName := range segments {
			result = append(result, common.VideoSegment{
				ID: segIDs[segmentName],
				Segment: hls.Segment{
					Name:      segmentName,
					StartTime: timestamp,
					EndTime:   timestamp.Add(segmentLength),
					Length:    segmentLength.Seconds(),
					URI:       fmt.Sprintf("file:///%s", segmentName),
				},
				SourceID: testSource.ID,
			})
			timestamp = timestamp.Add(segmentLength)
		}

		return result
	}
	testSegmentNames := []string{}
	testSegmentNameID := map[string]string{}
	for itr := 0; itr < segmentPerPlaylist; itr++ {
		testSegmentNames = append(testSegmentNames, uuid.NewString())
		testSegmentNameID[testSegmentNames[itr]] = uuid.NewString()
	}
	testSegments := defineSegmentList(startTime, testSegmentNameID, testSegmentNames)

	// Setup mocks
	mockDB.On(
		"GetLatestLiveStreamSegments",
		mock.AnythingOfType("*context.emptyCtx"),
		testSource.ID,
		segmentPerPlaylist,
	).Return(testSegments, nil).Once()

	playlist, err := uut.GetLiveStreamPlaylist(
		utCtxt,
		testSource,
		startTime.Add(segmentLength*time.Duration(segmentPerPlaylist)),
		true,
	)
	assert.Nil(err)
	assert.Equal(testSource.Name, playlist.Name)
	assert.Equal(startTime.Add(segmentLength*time.Duration(segmentPerPlaylist)), playlist.CreatedAt)
	assert.Equal(segmentLength.Seconds(), playlist.TargetSegDuration)
	assert.Len(playlist.Segments, segmentPerPlaylist)
	for idx, oneSegment := range playlist.Segments {
		testSegment := testSegments[idx]
		assert.EqualValues(testSegment.Segment, oneSegment)
	}

	// Verify the media sequence number
	referenceTime, err := vod.GetReferenceTime()
	assert.Nil(err)
	{
		timeDiff := startTime.Sub(referenceTime)
		timeDiffSec := int(timeDiff.Seconds())
		segLenSec := int(playlist.TargetSegDuration)
		mediaSequenceVal := timeDiffSec / segLenSec
		assert.Equal(mediaSequenceVal, *playlist.MediaSequenceVal)
	}
}

func TestBuildRecordingPlaylist(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()

	testSource := common.VideoSource{
		ID:                  uuid.NewString(),
		TargetSegmentLength: 5,
	}
	testRecording := common.Recording{
		ID:       uuid.NewString(),
		SourceID: testSource.ID,
	}
	testSegments := []common.VideoSegment{
		{ID: uuid.NewString(), SourceID: testSource.ID},
		{ID: uuid.NewString(), SourceID: testSource.ID},
		{ID: uuid.NewString(), SourceID: testSource.ID},
	}

	uut, err := vod.NewPlaylistBuilder(mockSQL, 1)
	assert.Nil(err)

	// Case 0: failed to find recording segments
	{
		// Prepare mock
		mockDB.On(
			"ListAllSegmentsOfRecording",
			mock.AnythingOfType("*context.emptyCtx"),
			testRecording.ID,
		).Return([]common.VideoSegment{}, fmt.Errorf("dummy error")).Once()

		_, err = uut.GetRecordingStreamPlaylist(utCtxt, testRecording)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}

	// Case 1: failed to find the associated source
	{
		// Prepare mock
		mockDB.On(
			"ListAllSegmentsOfRecording",
			mock.AnythingOfType("*context.emptyCtx"),
			testRecording.ID,
		).Return(testSegments, nil).Once()
		mockDB.On(
			"GetVideoSource",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(common.VideoSource{}, fmt.Errorf("dummy error")).Once()

		_, err = uut.GetRecordingStreamPlaylist(utCtxt, testRecording)
		assert.NotNil(err)
		assert.Equal("dummy error", err.Error())
	}
}
