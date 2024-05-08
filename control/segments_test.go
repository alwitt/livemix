package control_test

import (
	"context"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/control"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLiveSteamSegmentManagerRecordSegment(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockCache := mocks.NewVideoSegmentCache(t)

	uut, err := control.NewLiveStreamSegmentManager(utCtxt, mockSQL, mockCache, time.Minute, nil)
	assert.Nil(err)

	testSourceID := uuid.NewString()
	testSegmentID := uuid.NewString()
	testSegment := hls.Segment{Name: uuid.NewString()}
	testContent := []byte(uuid.NewString())

	// Setup mock
	mockDB.On(
		"RegisterLiveStreamSegment",
		mock.AnythingOfType("context.backgroundCtx"),
		testSourceID,
		testSegment,
	).Return(testSegmentID, nil).Once()
	mockDB.On(
		"GetLiveStreamSegment",
		mock.AnythingOfType("context.backgroundCtx"),
		testSegmentID,
	).Return(common.VideoSegment{ID: testSegmentID, SourceID: testSourceID}, nil).Once()
	mockCache.On(
		"CacheSegment",
		mock.AnythingOfType("context.backgroundCtx"),
		mock.AnythingOfType("common.VideoSegmentWithData"),
		time.Minute,
	).Run(func(args mock.Arguments) {
		cacheSegment := args.Get(1).(common.VideoSegmentWithData)
		assert.Equal(testSourceID, cacheSegment.SourceID)
		assert.EqualValues(testContent, cacheSegment.Content)
	}).Return(nil).Once()

	// Registry a segment
	assert.Nil(uut.RegisterLiveStreamSegment(utCtxt, testSourceID, testSegment, testContent))

	// Clean up
	assert.Nil(uut.Stop(utCtxt))
}
