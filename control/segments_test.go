package control_test

import (
	"context"
	"testing"
	"time"

	"github.com/alwitt/livemix/control"
	"github.com/alwitt/livemix/hls"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCentralSegmentManagerRecordSegment(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockSQL := mocks.NewConnectionManager(t)
	mockDB := mocks.NewPersistenceManager(t)
	mockSQL.On("NewPersistanceManager").Return(mockDB)
	mockDB.On("Close").Return()
	mockCache := mocks.NewVideoSegmentCache(t)

	uut, err := control.NewSegmentManager(utCtxt, mockSQL, mockCache, time.Minute)
	assert.Nil(err)

	testSourceID := uuid.NewString()
	testSegmentID := uuid.NewString()
	testSegment := hls.Segment{Name: uuid.NewString()}
	testContent := []byte(uuid.NewString())

	// Setup mock
	mockDB.On(
		"RegisterLiveStreamSegment",
		mock.AnythingOfType("*context.emptyCtx"),
		testSourceID,
		testSegment,
	).Return(testSegmentID, nil).Once()
	mockCache.On(
		"CacheSegment",
		mock.AnythingOfType("*context.emptyCtx"),
		testSegmentID,
		testContent,
		time.Minute,
	).Return(nil).Once()

	// Registry a segment
	assert.Nil(uut.RegisterLiveStreamSegment(utCtxt, testSourceID, testSegment, testContent))

	// Clean up
	assert.Nil(uut.Stop(utCtxt))
}
