package tracker_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/alwitt/livemix/tracker"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm/logger"
)

func TestSourceHLSMonitor(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)
	conns, err := db.NewSQLConnection(db.GetSqliteDialector(testDB, 20), logger.Info, false)
	assert.Nil(err)

	var testSource common.VideoSource
	{
		dbClient := conns.NewPersistanceManager()
		testSourceName := fmt.Sprintf("vid-%s.m3u8", uuid.NewString())
		testSourceID, err := dbClient.DefineVideoSource(utCtxt, testSourceName, 4, nil, nil)
		assert.Nil(err)
		testSource, err = dbClient.GetVideoSource(utCtxt, testSourceID)
		assert.Nil(err)
		dbClient.Close()
	}

	trackingWindow := time.Second * 15
	segmentLength := time.Second * 5

	mockSegReader := mocks.NewSegmentReader(t)
	testCache, err := utils.NewLocalVideoSegmentCache(utCtxt, time.Minute, nil)
	assert.Nil(err)

	segmentRX := make(chan common.VideoSegmentWithData)
	receiveSegCB := func(_ context.Context, segment common.VideoSegmentWithData) error {
		segmentRX <- segment
		return nil
	}

	// Define SourceHLSMonitor
	uut, err := tracker.NewSourceHLSMonitor(
		utCtxt, testSource, conns, trackingWindow, testCache, mockSegReader, receiveSegCB, nil, nil,
	)
	assert.Nil(err)

	// Helper function to define a playlist with segments
	definePlaylist := func(startTime time.Time, segments []string) hls.Playlist {
		// Define playlist
		result := hls.Playlist{
			Name:              testSource.Name,
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

	startTime := time.Now().UTC()

	segmentData := map[string][]byte{}
	segmentNameToID := map[string]string{}
	// Setup mocks ahead of time
	//
	// We expect five segments, so this should only be called five times
	mockSegReader.On(
		"ReadSegment",
		mock.AnythingOfType("*context.cancelCtx"),
		mock.AnythingOfType("common.VideoSegment"),
		mock.AnythingOfType("utils.SegmentReturnCallback"),
	).Run(func(args mock.Arguments) {
		// Parse the parameters
		segment := args.Get(1).(common.VideoSegment)
		segmentNameToID[segment.URI] = segment.ID
		log.Debugf(
			"=================== READING %s for %s ==================================",
			segment.URI,
			segment.ID,
		)

		// Trigger the callback to return the read "segment"
		returnCB := args.Get(2).(utils.SegmentReturnCallback)
		assert.Nil(returnCB(utCtxt, segment.ID, segmentData[segment.URI]))
	}).Return(nil).Times(5)

	// Case 0: registering new segments
	currentTime := startTime
	segmentNames0 := []string{}
	for itr := 0; itr < 3; itr++ {
		segmentNames0 = append(segmentNames0, fmt.Sprintf("seg-%s.ts", uuid.NewString()))
		segmentData[fmt.Sprintf("file:///%s", segmentNames0[itr])] = []byte(uuid.NewString())
	}
	playlist0 := definePlaylist(currentTime, segmentNames0)
	{
		// Send playlist to trigger update
		assert.Nil(uut.Update(utCtxt, playlist0, playlist0.CreatedAt))

		// Wait for responses
		lclCtxt, cancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		defer cancel()
		for itr := 0; itr < 3; itr++ {
			select {
			case <-lclCtxt.Done():
				assert.True(false, "timeout waiting for segments")
			case msg, ok := <-segmentRX:
				assert.True(ok)
				assert.Equal(segmentNames0[itr], msg.Name)
				assert.Equal(segmentNameToID[msg.URI], msg.ID)
				assert.EqualValues(segmentData[msg.URI], msg.Content)
			}
		}
	}
	// Check cache
	for _, segmentName := range segmentNames0 {
		segURL := fmt.Sprintf("file:///%s", segmentName)
		segID, ok := segmentNameToID[segURL]
		assert.True(ok)
		content, err := testCache.GetSegment(
			utCtxt, common.VideoSegment{ID: segID, SourceID: testSource.ID},
		)
		assert.Nil(err)
		assert.EqualValues(segmentData[segURL], content)
	}

	// Case 1: registering new segments, oldest should be purged
	currentTime = startTime
	segmentNames1 := make([]string, len(segmentNames0))
	copy(segmentNames1, segmentNames0)
	for itr := 3; itr < 5; itr++ {
		segmentNames1 = append(segmentNames1, fmt.Sprintf("seg-%s.ts", uuid.NewString()))
		segmentData[fmt.Sprintf("file:///%s", segmentNames1[itr])] = []byte(uuid.NewString())
	}
	playlist1 := definePlaylist(currentTime, segmentNames1)
	{
		// Send playlist to trigger update
		assert.Nil(uut.Update(utCtxt, playlist1, playlist1.CreatedAt))

		// Wait for responses
		lclCtxt, cancel := context.WithTimeout(utCtxt, time.Millisecond*50)
		defer cancel()
		for itr := 3; itr < 5; itr++ {
			select {
			case <-lclCtxt.Done():
				assert.True(false, "timeout waiting for segments")
			case msg, ok := <-segmentRX:
				assert.True(ok)
				assert.Equal(segmentNames1[itr], msg.Name)
				assert.Equal(segmentNameToID[msg.URI], msg.ID)
				assert.EqualValues(segmentData[msg.URI], msg.Content)
			}
		}
	}
	// Check cache
	for _, segmentName := range segmentNames1 {
		segURL := fmt.Sprintf("file:///%s", segmentName)
		segID, ok := segmentNameToID[segURL]
		assert.True(ok)
		content, err := testCache.GetSegment(
			utCtxt, common.VideoSegment{ID: segID, SourceID: testSource.ID},
		)
		assert.Nil(err)
		assert.EqualValues(segmentData[segURL], content)
	}

	// Clean up
	assert.Nil(uut.Stop(utCtxt))
}
