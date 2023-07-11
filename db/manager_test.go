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

	getURI := func(name string) *string {
		u := fmt.Sprintf("file:///%s.m3u8", name)
		return &u
	}

	// Case 1: create video source
	source1 := fmt.Sprintf("src-1-%s", uuid.NewString())
	sourceID1, err := uut.DefineVideoSource(utCtxt, source1, 2, getURI(source1), nil)
	assert.Nil(err)
	log.Debugf("Source ID1 %s", sourceID1)
	{
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(source1, entry.Name)
		assert.Equal(2, entry.TargetSegmentLength)
		assert.Equal(*getURI(source1), *entry.PlaylistURI)
		entry, err = uut.GetVideoSourceByName(utCtxt, source1)
		assert.Nil(err)
		assert.Equal(source1, entry.Name)
		assert.Equal(*getURI(source1), *entry.PlaylistURI)
	}

	// Case 2: create another with same name
	{
		_, err = uut.DefineVideoSource(utCtxt, source1, 2, getURI(source1), nil)
		assert.NotNil(err)
	}

	// Case 3: create another source
	source2 := fmt.Sprintf("src-2-%s", uuid.NewString())
	sourceID2, err := uut.DefineVideoSource(utCtxt, source2, 2, getURI(source2), nil)
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
		assert.Equal(*getURI(source2), *entry.PlaylistURI)
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
		assert.Equal(*getURI(source1), *entry.PlaylistURI)
		assert.Equal(-1, entry.Streaming)
	}
	{
		assert.Nil(uut.ChangeVideoSourceStreamState(utCtxt, sourceID1, 1))
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(1, entry.Streaming)
		assert.Nil(entry.ReqRespTargetID)
	}
	{
		assert.Nil(uut.ChangeVideoSourceStreamState(utCtxt, sourceID1, -1))
		reqRespID := uuid.NewString()
		timestamp := time.Now().UTC()
		assert.Nil(uut.RefreshVideoSourceStats(utCtxt, sourceID1, reqRespID, timestamp))
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(-1, entry.Streaming)
		assert.Equal(reqRespID, *entry.ReqRespTargetID)
		assert.Equal(timestamp, entry.SourceLocalTime)
	}

	// Case 5: recreate existing entry
	source3 := fmt.Sprintf("src-3-%s", uuid.NewString())
	assert.Nil(uut.RecordKnownVideoSource(utCtxt, sourceID2, source3, 4, getURI(source3), nil, 1))
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
		assert.Equal(4, entry.TargetSegmentLength)
		assert.Equal(*getURI(source3), *entry.PlaylistURI)
		assert.Equal(1, entry.Streaming)
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
		assert.Equal(*getURI(source3), *entry.PlaylistURI)
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
		utCtxt, uuid.NewString(), 4, nil, nil,
	)
	assert.Nil(err)

	// Case 0: no segments
	{
		_, err := uut.GetLiveStreamSegment(utCtxt, uuid.NewString())
		assert.NotNil(err)
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Empty(entries)
	}

	startTime := time.Now().UTC()
	segDuration := time.Second * 4

	// Case 1: register new segment
	segment0 := fmt.Sprintf("seg-0-%s.ts", uuid.NewString())
	segStart0 := startTime
	segStop0 := segStart0.Add(segDuration)
	segmentID0, err := uut.RegisterLiveStreamSegment(utCtxt, sourceID, hls.Segment{
		Name:      segment0,
		StartTime: segStart0,
		EndTime:   segStop0,
		Length:    segDuration.Seconds(),
		URI:       fmt.Sprintf("file:///%s", segment0),
	})
	assert.Nil(err)
	{
		seg, err := uut.GetLiveStreamSegmentByName(utCtxt, segment0)
		assert.Nil(err)
		assert.Equal(segment0, seg.Name)
		assert.Equal(segStart0, seg.StartTime)
		assert.Equal(segStop0, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment0), seg.URI)
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
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
	segmentID1, err := uut.RegisterLiveStreamSegment(utCtxt, sourceID, hls.Segment{
		Name:      segment1,
		StartTime: segStart1,
		EndTime:   segStop1,
		Length:    segDuration.Seconds(),
		URI:       fmt.Sprintf("file:///%s", segment1),
	})
	assert.Nil(err)
	{
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
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

	// Case 3: fetch segment conditionally
	// By time
	{
		targetTime := startTime.Add(segDuration).Add(segDuration / 2)
		entries, err := uut.ListAllLiveStreamSegmentsAfterTime(utCtxt, sourceID, targetTime)
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
		entries, err := uut.ListAllLiveStreamSegmentsAfterTime(utCtxt, sourceID, targetTime)
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
	// Get latest
	{
		entries, err := uut.GetLatestLiveStreamSegments(utCtxt, sourceID, 2)
		assert.Nil(err)
		assert.Len(entries, 2)
		assert.Equal(segmentID0, entries[0].ID)
		assert.Equal(segmentID1, entries[1].ID)
		entries, err = uut.GetLatestLiveStreamSegments(utCtxt, sourceID, 1)
		assert.Nil(err)
		assert.Len(entries, 1)
		assert.Equal(segmentID1, entries[0].ID)
		assert.Equal(segment1, entries[0].Name)
		assert.Equal(segStart1, entries[0].StartTime)
		assert.Equal(segStop1, entries[0].EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment1), entries[0].URI)
	}

	// Case 4: delete segment
	assert.Nil(uut.DeleteLiveStreamSegment(utCtxt, segmentID1))
	{
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
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
	segIDs, err := uut.BulkRegisterLiveStreamSegments(utCtxt, sourceID, segmentEntries)
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
		assert.Nil(uut.BulkDeleteLiveStreamSegment(utCtxt, deleteIDs))
	}
	{
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 1)
		assert.Equal(segmentID0, entries[0].ID)
	}

	// Case #: delete video source
	assert.Nil(uut.DeleteVideoSource(utCtxt, sourceID))
	{
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 0)
	}
}

func TestDBManagerVideoSegmentPurgeOldSegments(t *testing.T) {
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
		utCtxt, uuid.NewString(), 4, nil, nil,
	)
	assert.Nil(err)

	currentTime := time.Now().UTC()
	segmentLength := time.Second * 10

	// Define test segments
	testSegments := []hls.Segment{}
	for itr := 0; itr < 6; itr++ {
		segmentName := fmt.Sprintf("ut-seg-%s.ts", uuid.NewString())
		testSegments = append(testSegments, hls.Segment{
			Name:      segmentName,
			StartTime: currentTime.Add(segmentLength * time.Duration(itr)),
			EndTime:   currentTime.Add(segmentLength * time.Duration(itr+1)),
			Length:    segmentLength.Seconds(),
			URI:       fmt.Sprintf("file:///tmp/%s", segmentName),
		})
	}
	// Install segments
	_, err = uut.BulkRegisterLiveStreamSegments(utCtxt, sourceID, testSegments)
	assert.Nil(err)

	// Case 0: read back segments
	{
		segments, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(segments, 6)
		for idx, segment := range segments {
			assert.Equal(testSegments[idx].Name, segment.Name)
		}
	}

	// Case 1: delete segments older than time
	{
		markTime := currentTime.Add(segmentLength * 3)
		assert.Nil(uut.PurgeOldLiveStreamSegments(utCtxt, markTime))
		segments, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(segments, 4)
		for idx, segment := range segments {
			assert.Equal(testSegments[idx+2].Name, segment.Name)
		}
	}

	// Case 2: delete segments older than time
	{
		markTime := currentTime.Add(segmentLength * 6)
		assert.Nil(uut.PurgeOldLiveStreamSegments(utCtxt, markTime))
		segments, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(segments, 1)
		assert.Equal(testSegments[5].Name, segments[0].Name)
	}
}
