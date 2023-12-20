package tracker_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/alwitt/livemix/tracker"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSourceHLSTrackerUpdate(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()

	testSource := common.VideoSource{
		ID:   uuid.NewString(),
		Name: fmt.Sprintf("vid-%s.m3u8", uuid.NewString()),
	}
	testSource.PlaylistURI = func() *string {
		t := fmt.Sprintf("file:///%s", testSource.Name)
		return &t
	}()

	trackingWindow := time.Second * 15

	uut, err := tracker.NewSourceHLSTracker(testSource, mockSQL, trackingWindow)
	assert.Nil(err)

	utCtxt := context.Background()

	segmentLength := time.Second * 5

	// Helper function to define a playlist with segments
	definePlaylist := func(name string, startTime time.Time, segments []string) hls.Playlist {
		// Define playlist
		result := hls.Playlist{
			Name:              name,
			CreatedAt:         startTime.Add(segmentLength * (time.Duration(len(segments)))),
			Version:           3,
			TargetSegDuration: segmentLength.Seconds(),
			Segments:          []hls.Segment{},
		}

		// Define segments
		timestamp := startTime
		for _, segmentName := range segments {
			result.Segments = append(result.Segments, hls.Segment{
				Name:      segmentName,
				StartTime: timestamp,
				EndTime:   timestamp.Add(segmentLength),
				Length:    segmentLength.Seconds(),
				URI:       fmt.Sprintf("file:///%s", segmentName),
			})
			timestamp = timestamp.Add(segmentLength)
		}

		return result
	}

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

	startTime := time.Now().UTC()

	// Case 0: registering new segments, but name or URI are wrong
	currentTime := startTime
	segmentNames0 := []string{}
	segmentIDs0 := map[string]string{}
	for itr := 0; itr < 3; itr++ {
		segmentName := fmt.Sprintf("seg-%s.ts", uuid.NewString())
		segmentNames0 = append(segmentNames0, segmentName)
		segmentIDs0[segmentName] = ulid.Make().String()
	}
	{
		playlist0 := definePlaylist(uuid.NewString(), currentTime, segmentNames0)
		_, err := uut.Update(utCtxt, playlist0, playlist0.CreatedAt)
		assert.NotNil(err)
	}

	// Case 1: registering new segments, with no previous segments
	{
		playlist0 := definePlaylist(testSource.Name, currentTime, segmentNames0)

		// Setup mocks
		mockDB.On(
			"ListAllLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return([]common.VideoSegment{}, nil).Once()
		mockDB.On(
			"BulkRegisterLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
			mock.AnythingOfType("[]hls.Segment"),
		).Run(func(args mock.Arguments) {
			segList := args.Get(2).([]hls.Segment)
			assert.Len(segList, 3)
			assert.EqualValues(playlist0.Segments, segList)
		}).Return(segmentIDs0, nil).Once()
		mockDB.On(
			"ListAllLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(defineSegmentList(currentTime, segmentIDs0, segmentNames0), nil).Once()
		mockDB.On(
			"DeleteOldLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			mock.AnythingOfType("time.Time"),
		).Return(nil).Once()

		newSegs, err := uut.Update(utCtxt, playlist0, playlist0.CreatedAt)
		assert.Nil(err)
		assert.Len(newSegs, 3)
		// Verify the returned segments
		for _, oneSeg := range newSegs {
			segID, ok := segmentIDs0[oneSeg.Name]
			assert.True(ok)
			assert.Equal(segID, oneSeg.ID)
		}
	}

	// Case 2: registering new segments, with previous segments
	segmentNames1 := make([]string, len(segmentNames0))
	segmentIDs1 := map[string]string{}
	{
		copy(segmentNames1, segmentNames0)
		segmentName := fmt.Sprintf("seg-%s.ts", uuid.NewString())
		segmentNames1 = append(segmentNames1, segmentName)
		segmentIDs1[segmentName] = ulid.Make().String()
		// Update segmentIDs1 with values from segmentIDs0
		for segName, segID := range segmentIDs0 {
			segmentIDs1[segName] = segID
		}

		playlist1 := definePlaylist(testSource.Name, currentTime, segmentNames1)
		assert.Len(playlist1.Segments, 4)

		// Setup mocks
		mockDB.On(
			"ListAllLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(defineSegmentList(currentTime, segmentIDs0, segmentNames0), nil).Once()
		mockDB.On(
			"BulkRegisterLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
			mock.AnythingOfType("[]hls.Segment"),
		).Run(func(args mock.Arguments) {
			segList := args.Get(2).([]hls.Segment)
			assert.Len(segList, 1)
			assert.EqualValues(playlist1.Segments[3], segList[0])
		}).Return(map[string]string{segmentName: segmentIDs1[segmentName]}, nil).Once()
		mockDB.On(
			"ListAllLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(defineSegmentList(currentTime, segmentIDs1, segmentNames1), nil).Once()
		mockDB.On(
			"DeleteOldLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			mock.AnythingOfType("time.Time"),
		).Return(nil).Once()

		newSegs, err := uut.Update(utCtxt, playlist1, playlist1.CreatedAt)
		assert.Nil(err)
		assert.Len(newSegs, 1)
		// Verify the returned segments
		for _, oneSeg := range newSegs {
			segID, ok := segmentIDs1[oneSeg.Name]
			assert.True(ok)
			assert.Equal(segID, oneSeg.ID)
		}
	}

	// Case 3: registering new segments, with previous segments, oldest segment timeout
	segmentNames2 := make([]string, len(segmentNames1))
	segmentIDs2 := map[string]string{}
	{
		copy(segmentNames2, segmentNames1)
		segmentName := fmt.Sprintf("seg-%s.ts", uuid.NewString())
		segmentNames2 = append(segmentNames2, segmentName)
		segmentIDs2[segmentName] = ulid.Make().String()
		// Update segmentIDs2 with values from segmentIDs0
		for segName, segID := range segmentIDs1 {
			segmentIDs2[segName] = segID
		}

		playlist2 := definePlaylist(testSource.Name, currentTime, segmentNames2)
		assert.Len(playlist2.Segments, 5)

		// Setup mocks
		mockDB.On(
			"ListAllLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(defineSegmentList(currentTime, segmentIDs1, segmentNames1), nil).Once()
		mockDB.On(
			"BulkRegisterLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
			mock.AnythingOfType("[]hls.Segment"),
		).Run(func(args mock.Arguments) {
			segList := args.Get(2).([]hls.Segment)
			assert.Len(segList, 1)
			assert.EqualValues(playlist2.Segments[4], segList[0])
		}).Return(map[string]string{segmentName: segmentIDs2[segmentName]}, nil).Once()
		mockDB.On(
			"ListAllLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			testSource.ID,
		).Return(defineSegmentList(currentTime, segmentIDs2, segmentNames2), nil).Once()
		mockDB.On(
			"DeleteOldLiveStreamSegments",
			mock.AnythingOfType("*context.emptyCtx"),
			mock.AnythingOfType("time.Time"),
		).Return(nil).Once()

		newSegs, err := uut.Update(utCtxt, playlist2, playlist2.CreatedAt)
		assert.Nil(err)
		assert.Len(newSegs, 1)
		// Verify the returned segments
		for _, oneSeg := range newSegs {
			segID, ok := segmentIDs2[oneSeg.Name]
			assert.True(ok)
			assert.Equal(segID, oneSeg.ID)
		}
	}
}
