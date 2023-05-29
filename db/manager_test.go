package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/logger"
)

func TestDBManagerVideoSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)
	uut, err := db.NewManager(db.GetSqliteDialector(testDB), logger.Info)
	assert.Nil(err)

	log.Debugf("Using %s", testDB)

	utCtxt := context.Background()

	assert.Nil(uut.Ready(utCtxt))

	// Case 0: no sources
	{
		_, err := uut.GetVideoSource(utCtxt, uuid.NewString())
		assert.NotNil(err)
		result, err := uut.ListVideoSources(utCtxt)
		assert.Nil(err)
		assert.Len(result, 0)
	}

	getURI := func(name string) string {
		return fmt.Sprintf("file:///%s.m3u8", name)
	}

	// Case 1: create video source
	source1 := fmt.Sprintf("src-1-%s", uuid.NewString())
	sourceID1, err := uut.DefineVideoSource(utCtxt, source1, getURI(source1), nil)
	assert.Nil(err)
	log.Debugf("Source ID1 %s", sourceID1)
	{
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(source1, entry.Name)
		assert.Equal(getURI(source1), entry.PlaylistURI)
		entry, err = uut.GetVideoSourceByName(utCtxt, source1)
		assert.Nil(err)
		assert.Equal(source1, entry.Name)
		assert.Equal(getURI(source1), entry.PlaylistURI)
	}

	// Case 2: create another with same name
	{
		_, err = uut.DefineVideoSource(utCtxt, source1, getURI(source1), nil)
		assert.NotNil(err)
	}

	// Case 3: create another source
	source2 := fmt.Sprintf("src-2-%s", uuid.NewString())
	sourceID2, err := uut.DefineVideoSource(utCtxt, source2, getURI(source2), nil)
	assert.Nil(err)
	log.Debugf("Source ID2 %s", sourceID2)
	{
		entries, err := uut.ListVideoSources(utCtxt)
		assert.Nil(err)
		asMap := map[string]common.VideoSource{}
		for _, entry := range entries {
			asMap[entry.ID] = entry
		}
		assert.Len(asMap, 2)
		assert.Contains(asMap, sourceID1)
		assert.Contains(asMap, sourceID2)
		entry, ok := asMap[sourceID2]
		assert.True(ok)
		assert.Equal(source2, entry.Name)
		assert.Equal(getURI(source2), entry.PlaylistURI)
	}

	// Case 4: update entry
	newName := fmt.Sprintf("src-new-%s", uuid.NewString())
	assert.Nil(uut.UpdateVideoSource(
		utCtxt, common.VideoSource{ID: sourceID1, Name: newName, PlaylistURI: getURI(source1)},
	))
	{
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(newName, entry.Name)
		assert.Equal(getURI(source1), entry.PlaylistURI)
	}

	// Case 5: recreate existing entry
	source3 := fmt.Sprintf("src-3-%s", uuid.NewString())
	assert.Nil(uut.RecordKnownVideoSource(utCtxt, sourceID2, source3, getURI(source3), nil))
	{
		entries, err := uut.ListVideoSources(utCtxt)
		assert.Nil(err)
		asMap := map[string]common.VideoSource{}
		for _, entry := range entries {
			asMap[entry.ID] = entry
		}
		assert.Len(asMap, 2)
		assert.Contains(asMap, sourceID2)
		entry, ok := asMap[sourceID2]
		assert.True(ok)
		assert.Equal(source3, entry.Name)
		assert.Equal(getURI(source3), entry.PlaylistURI)
	}

	// Case 6: delete entry
	assert.Nil(uut.DeleteVideoSource(utCtxt, sourceID1))
	{
		_, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.NotNil(err)
	}
	{
		entries, err := uut.ListVideoSources(utCtxt)
		assert.Nil(err)
		asMap := map[string]common.VideoSource{}
		for _, entry := range entries {
			asMap[entry.ID] = entry
		}
		assert.Len(asMap, 1)
		assert.Contains(asMap, sourceID2)
		entry, ok := asMap[sourceID2]
		assert.True(ok)
		assert.Equal(source3, entry.Name)
		assert.Equal(getURI(source3), entry.PlaylistURI)
	}
}

func TestDBManagerVideoSegment(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)
	uut, err := db.NewManager(db.GetSqliteDialector(testDB), logger.Info)
	assert.Nil(err)

	log.Debugf("Using %s", testDB)

	utCtxt := context.Background()

	assert.Nil(uut.Ready(utCtxt))

	// Create a source
	sourceID, err := uut.DefineVideoSource(
		utCtxt, uuid.NewString(), fmt.Sprintf("file:///%s.m3u8", uuid.NewString()), nil,
	)
	assert.Nil(err)

	// Case 0: no segments
	{
		_, err := uut.GetSegment(utCtxt, uuid.NewString())
		assert.NotNil(err)
		entries, err := uut.ListAllSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Empty(entries)
	}

	startTime := time.Now().UTC()
	segDuration := time.Second * 4

	// Case 1: register new segment
	segment0 := fmt.Sprintf("seg-0-%s.ts", uuid.NewString())
	segStart0 := startTime
	segStop0 := segStart0.Add(segDuration)
	segmentID0, err := uut.RegisterSegment(utCtxt, sourceID, hls.Segment{
		Name:      segment0,
		StartTime: segStart0,
		EndTime:   segStop0,
		Length:    segDuration.Seconds(),
		URI:       fmt.Sprintf("file:///%s", segment0),
	})
	assert.Nil(err)
	{
		seg, err := uut.GetSegment(utCtxt, segmentID0)
		assert.Nil(err)
		assert.Equal(segment0, seg.Name)
		assert.Equal(segStart0, seg.StartTime)
		assert.Equal(segStop0, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment0), seg.URI)
		entries, err := uut.ListAllSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 1)
		seg = entries[0]
		assert.Equal(segment0, seg.Name)
		assert.Equal(segStart0, seg.StartTime)
		assert.Equal(segStop0, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment0), seg.URI)
	}

	// Case 2: register new segment
	segment1 := fmt.Sprintf("seg-1-%s.ts", uuid.NewString())
	segStart1 := segStop0
	segStop1 := segStart1.Add(segDuration)
	segmentID1, err := uut.RegisterSegment(utCtxt, sourceID, hls.Segment{
		Name:      segment1,
		StartTime: segStart1,
		EndTime:   segStop1,
		Length:    segDuration.Seconds(),
		URI:       fmt.Sprintf("file:///%s", segment1),
	})
	assert.Nil(err)
	{
		entries, err := uut.ListAllSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 2)
		segMap := map[string]common.VideoSegment{}
		for _, segment := range entries {
			segMap[segment.ID] = segment
		}
		assert.Contains(segMap, segmentID1)
		seg := segMap[segmentID1]
		assert.Equal(segment1, seg.Name)
		assert.Equal(segStart1, seg.StartTime)
		assert.Equal(segStop1, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment1), seg.URI)
	}

	// Case 3: fetch segment by timestamp
	{
		targetTime := startTime.Add(segDuration).Add(segDuration / 2)
		entries, err := uut.ListAllSegmentsAfterTime(utCtxt, sourceID, targetTime)
		assert.Nil(err)
		assert.Len(entries, 1)
		segMap := map[string]common.VideoSegment{}
		for _, segment := range entries {
			segMap[segment.ID] = segment
		}
		assert.Contains(segMap, segmentID1)
		seg := segMap[segmentID1]
		assert.Equal(segment1, seg.Name)
		assert.Equal(segStart1, seg.StartTime)
		assert.Equal(segStop1, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment1), seg.URI)
	}
	{
		targetTime := startTime.Add(segDuration / 2)
		entries, err := uut.ListAllSegmentsAfterTime(utCtxt, sourceID, targetTime)
		assert.Nil(err)
		assert.Len(entries, 2)
		segMap := map[string]common.VideoSegment{}
		for _, segment := range entries {
			segMap[segment.ID] = segment
		}
		assert.Contains(segMap, segmentID0)
		assert.Contains(segMap, segmentID1)
		seg := segMap[segmentID0]
		assert.Equal(segment0, seg.Name)
		assert.Equal(segStart0, seg.StartTime)
		assert.Equal(segStop0, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment0), seg.URI)
	}
	{
		targetTime := startTime.Add(segDuration).Add(segDuration / 2)
		entries, err := uut.ListAllSegmentsBeforeTime(utCtxt, sourceID, targetTime)
		assert.Nil(err)
		assert.Len(entries, 1)
		segMap := map[string]common.VideoSegment{}
		for _, segment := range entries {
			segMap[segment.ID] = segment
		}
		assert.Contains(segMap, segmentID0)
		seg := segMap[segmentID0]
		assert.Equal(segment0, seg.Name)
		assert.Equal(segStart0, seg.StartTime)
		assert.Equal(segStop0, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment0), seg.URI)
	}
	{
		targetTime := startTime.Add(segDuration * 2).Add(segDuration / 2)
		entries, err := uut.ListAllSegmentsBeforeTime(utCtxt, sourceID, targetTime)
		assert.Nil(err)
		assert.Len(entries, 2)
		segMap := map[string]common.VideoSegment{}
		for _, segment := range entries {
			segMap[segment.ID] = segment
		}
		assert.Contains(segMap, segmentID0)
		assert.Contains(segMap, segmentID1)
	}

	// Case 4: delete segment
	assert.Nil(uut.DeleteSegment(utCtxt, segmentID1))
	{
		entries, err := uut.ListAllSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 1)
		seg := entries[0]
		assert.Equal(segment0, seg.Name)
		assert.Equal(segStart0, seg.StartTime)
		assert.Equal(segStop0, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment0), seg.URI)
	}

	// Case 5: batch create segments
	segments := []string{
		fmt.Sprintf("seg-2-%s.ts", uuid.NewString()),
		fmt.Sprintf("seg-3-%s.ts", uuid.NewString()),
	}
	segmentEntries := []hls.Segment{}
	segStart := segStop1
	segStop := segStart.Add(segDuration)
	for _, segName := range segments {
		segmentEntries = append(segmentEntries, hls.Segment{
			Name:      segName,
			StartTime: segStart,
			EndTime:   segStop,
			Length:    segDuration.Seconds(),
			URI:       fmt.Sprintf("file:///%s", segName),
		})
		segStart = segStop
		segStop = segStart.Add(segDuration)
	}
	segIDs, err := uut.BulkRegisterSegments(utCtxt, sourceID, segmentEntries)
	assert.Nil(err)
	assert.Len(segIDs, 2)
	for _, segName := range segments {
		_, ok := segIDs[segName]
		assert.True(ok)
	}

	// Case 6: batch delete segments
	{
		deleteIDs := []string{}
		for _, segID := range segIDs {
			deleteIDs = append(deleteIDs, segID)
		}
		assert.Nil(uut.BulkDeleteSegment(utCtxt, deleteIDs))
	}
	{
		entries, err := uut.ListAllSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 1)
		assert.Equal(segmentID0, entries[0].ID)
	}

	// Case #: delete video source
	assert.Nil(uut.DeleteVideoSource(utCtxt, sourceID))
	{
		entries, err := uut.ListAllSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 0)
	}
}