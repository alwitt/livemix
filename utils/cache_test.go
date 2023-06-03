package utils

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestLocalSegmentCacheBasicSanity(t *testing.T) {
	assert := assert.New(t)

	utCtxt := context.Background()

	uut, err := NewLocalVideoSegmentCache(utCtxt, time.Minute)
	assert.Nil(err)

	// Case 0: no segments cached
	{
		_, err := uut.GetLiveStreamSegment(utCtxt, uuid.NewString())
		assert.NotNil(err)
	}

	// Case 1: add segment
	segment0 := uuid.NewString()
	content0 := []byte(uuid.NewString())
	assert.Nil(uut.CacheSegment(utCtxt, segment0, content0, time.Second))
	{
		content, err := uut.GetLiveStreamSegment(utCtxt, segment0)
		assert.Nil(err)
		assert.Equal(content0, content)
		_, err = uut.GetLiveStreamSegment(utCtxt, uuid.NewString())
		assert.NotNil(err)
	}

	// Case 2: update segment content
	content1 := []byte(uuid.NewString())
	assert.Nil(uut.CacheSegment(utCtxt, segment0, content1, time.Second))
	{
		content, err := uut.GetLiveStreamSegment(utCtxt, segment0)
		assert.Nil(err)
		assert.Equal(content1, content)
	}

	// Case 3: add segment
	segment2 := uuid.NewString()
	content2 := []byte(uuid.NewString())
	assert.Nil(uut.CacheSegment(utCtxt, segment2, content2, time.Second))
	{
		content, err := uut.GetLiveStreamSegment(utCtxt, segment2)
		assert.Nil(err)
		assert.Equal(content2, content)
	}

	// Case 4: delete from cache
	assert.Nil(uut.PurgeSegments(utCtxt, []string{segment0}))
	{
		checkIDs := []string{segment0, segment2}
		cached, err := uut.GetSegments(utCtxt, checkIDs)
		assert.Nil(err)
		assert.Len(cached, 1)
		assert.Contains(cached, segment2)
	}
}

func TestLocalSegmentCacheManualCachePurgeTrigger(t *testing.T) {
	assert := assert.New(t)

	utCtxt := context.Background()

	uut, err := NewLocalVideoSegmentCache(utCtxt, time.Minute)
	assert.Nil(err)

	startTime := time.Now().UTC()

	// Setup test entries
	entry0 := uuid.NewString()
	ttl0 := time.Second
	entry1 := uuid.NewString()
	ttl1 := time.Second * 2
	entry2 := uuid.NewString()
	ttl2 := time.Second * 4
	assert.Nil(uut.CacheSegment(utCtxt, entry0, []byte(uuid.NewString()), ttl0))
	assert.Nil(uut.CacheSegment(utCtxt, entry1, []byte(uuid.NewString()), ttl1))
	assert.Nil(uut.CacheSegment(utCtxt, entry2, []byte(uuid.NewString()), ttl2))

	uutCast, ok := uut.(*inProcessSegmentCacheImpl)
	assert.True(ok)

	purgeTime := startTime
	assert.Nil(uutCast.purgeExpiredEntry(utCtxt, purgeTime))
	{
		entries, err := uut.GetSegments(utCtxt, []string{entry0, entry1, entry2})
		assert.Nil(err)
		assert.Len(entries, 3)
	}

	purgeTime = purgeTime.Add(time.Millisecond * 1250)
	assert.Nil(uutCast.purgeExpiredEntry(utCtxt, purgeTime))
	{
		entries, err := uut.GetSegments(utCtxt, []string{entry0, entry1, entry2})
		assert.Nil(err)
		assert.Len(entries, 2)
		_, err = uut.GetLiveStreamSegment(utCtxt, entry0)
		assert.NotNil(err)
	}

	purgeTime = purgeTime.Add(time.Millisecond * 2500)
	assert.Nil(uutCast.purgeExpiredEntry(utCtxt, purgeTime))
	{
		entries, err := uut.GetSegments(utCtxt, []string{entry0, entry1, entry2})
		assert.Nil(err)
		assert.Len(entries, 1)
		_, err = uut.GetLiveStreamSegment(utCtxt, entry0)
		assert.NotNil(err)
		_, err = uut.GetLiveStreamSegment(utCtxt, entry1)
		assert.NotNil(err)
		_, err = uut.GetLiveStreamSegment(utCtxt, entry2)
		assert.Nil(err)
	}
}

func TestLocalSegmentCacheAutoCachePurge(t *testing.T) {
	assert := assert.New(t)

	utCtxt := context.Background()

	uut, err := NewLocalVideoSegmentCache(utCtxt, time.Millisecond*50)
	assert.Nil(err)

	// Setup test entries
	entry0 := uuid.NewString()
	ttl0 := time.Millisecond * 25
	entry1 := uuid.NewString()
	ttl1 := time.Millisecond * 70
	entry2 := uuid.NewString()
	ttl2 := time.Millisecond * 120
	assert.Nil(uut.CacheSegment(utCtxt, entry0, []byte(uuid.NewString()), ttl0))
	assert.Nil(uut.CacheSegment(utCtxt, entry1, []byte(uuid.NewString()), ttl1))
	assert.Nil(uut.CacheSegment(utCtxt, entry2, []byte(uuid.NewString()), ttl2))

	// Verify all entry are in place
	{
		entries, err := uut.GetSegments(utCtxt, []string{entry0, entry1, entry2})
		assert.Nil(err)
		assert.Len(entries, 3)
	}

	// Verify first entry is gone
	time.Sleep(time.Millisecond * 60)
	{
		entries, err := uut.GetSegments(utCtxt, []string{entry0, entry1, entry2})
		assert.Nil(err)
		assert.Len(entries, 2)
		_, err = uut.GetLiveStreamSegment(utCtxt, entry0)
		assert.NotNil(err)
	}

	// Verify second entry is gone
	time.Sleep(time.Millisecond * 45)
	{
		entries, err := uut.GetSegments(utCtxt, []string{entry0, entry1, entry2})
		assert.Nil(err)
		assert.Len(entries, 1)
		_, err = uut.GetLiveStreamSegment(utCtxt, entry0)
		assert.NotNil(err)
		_, err = uut.GetLiveStreamSegment(utCtxt, entry1)
		assert.NotNil(err)
		_, err = uut.GetLiveStreamSegment(utCtxt, entry2)
		assert.Nil(err)
	}
}
