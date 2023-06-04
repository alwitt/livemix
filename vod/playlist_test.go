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

	mockDB := mocks.NewPersistenceManager(t)

	testSource := common.VideoSource{
		ID:   uuid.NewString(),
		Name: fmt.Sprintf("vid-%s.m3u8", uuid.NewString()),
	}
	testSource.PlaylistURI = fmt.Sprintf("file:///%s", testSource.Name)

	segmentLength := time.Second * 5

	segmentPerPlaylist := 3

	uut, err := vod.NewPlaylistBuilder(mockDB, segmentLength, segmentPerPlaylist)
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
		utCtxt, testSource, startTime.Add(segmentLength*time.Duration(segmentPerPlaylist)),
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
}
