package db_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm/logger"
)

func TestDBManagerVideoSource(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)
	conns, err := db.NewSQLConnection(db.GetSqliteDialector(testDB, 20), logger.Info, false)
	assert.Nil(err)
	assert.Nil(conns.ApplySQLitePragmas(common.SqliteConfig{BusyTimeoutMSec: 20}))

	log.Debugf("Using %s", testDB)

	utCtxt := context.Background()

	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.Ready(utCtxt))
		uut.Close()
	}

	// Case 0: no sources
	{
		uut := conns.NewPersistanceManager()
		_, err := uut.GetVideoSource(utCtxt, uuid.NewString())
		assert.NotNil(err)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		result, err := uut.ListVideoSources(utCtxt)
		assert.Nil(err)
		assert.Len(result, 0)
		uut.Close()
	}

	getURI := func(name string) *string {
		u := fmt.Sprintf("file:///%s.m3u8", name)
		return &u
	}

	// Case 1: create video source
	source1 := fmt.Sprintf("src-1-%s", uuid.NewString())
	var sourceID1 string
	{
		uut := conns.NewPersistanceManager()
		sourceID1, err = uut.DefineVideoSource(utCtxt, source1, 2, getURI(source1), nil)
		assert.Nil(err)
		log.Debugf("Source ID1 %s", sourceID1)
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(source1, entry.Name)
		assert.Equal(2, entry.TargetSegmentLength)
		assert.Equal(*getURI(source1), *entry.PlaylistURI)
		entry, err = uut.GetVideoSourceByName(utCtxt, source1)
		assert.Nil(err)
		assert.Equal(source1, entry.Name)
		assert.Equal(*getURI(source1), *entry.PlaylistURI)
		uut.Close()
	}

	// Case 2: create another with same name
	{
		uut := conns.NewPersistanceManager()
		_, err = uut.DefineVideoSource(utCtxt, source1, 2, getURI(source1), nil)
		assert.NotNil(err)
		uut.Close()
	}

	// Case 3: create another source
	source2 := fmt.Sprintf("src-2-%s", uuid.NewString())
	var sourceID2 string
	{
		uut := conns.NewPersistanceManager()
		sourceID2, err = uut.DefineVideoSource(utCtxt, source2, 2, getURI(source2), nil)
		assert.Nil(err)
		log.Debugf("Source ID2 %s", sourceID2)
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
		uut.Close()
	}

	// Case 4: update entry
	newName := fmt.Sprintf("src-new-%s", uuid.NewString())
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.UpdateVideoSource(
			utCtxt, common.VideoSource{ID: sourceID1, Name: newName, PlaylistURI: getURI(source1)},
		))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(newName, entry.Name)
		assert.Equal(*getURI(source1), *entry.PlaylistURI)
		assert.Equal(-1, entry.Streaming)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.ChangeVideoSourceStreamState(utCtxt, sourceID1, 1))
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(1, entry.Streaming)
		assert.Nil(entry.ReqRespTargetID)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.ChangeVideoSourceStreamState(utCtxt, sourceID1, -1))
		reqRespID := uuid.NewString()
		timestamp := time.Now().UTC()
		assert.Nil(uut.UpdateVideoSourceStats(utCtxt, sourceID1, reqRespID, timestamp))
		entry, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.Nil(err)
		assert.Equal(-1, entry.Streaming)
		assert.Equal(reqRespID, *entry.ReqRespTargetID)
		assert.Equal(timestamp, entry.SourceLocalTime)
		uut.Close()
	}

	// Case 5: recreate existing entry
	source3 := fmt.Sprintf("src-3-%s", uuid.NewString())
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.RecordKnownVideoSource(utCtxt, sourceID2, source3, 4, getURI(source3), nil, 1))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
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
		uut.Close()
	}

	// Case 6: delete entry
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.DeleteVideoSource(utCtxt, sourceID1))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		_, err := uut.GetVideoSource(utCtxt, sourceID1)
		assert.NotNil(err)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
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
		uut.Close()
	}
}

func TestDBManagerVideoSegment(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)
	conns, err := db.NewSQLConnection(db.GetSqliteDialector(testDB, 20), logger.Info, false)
	assert.Nil(err)

	log.Debugf("Using %s", testDB)

	utCtxt := context.Background()

	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.Ready(utCtxt))
		uut.Close()
	}

	// Create a source
	var sourceID string
	{
		uut := conns.NewPersistanceManager()
		sourceID, err = uut.DefineVideoSource(utCtxt, uuid.NewString(), 4, nil, nil)
		assert.Nil(err)
		uut.Close()
	}

	// Case 0: no segments
	{
		uut := conns.NewPersistanceManager()
		_, err := uut.GetLiveStreamSegment(utCtxt, uuid.NewString())
		assert.NotNil(err)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Empty(entries)
		uut.Close()
	}

	startTime := time.Now().UTC()
	segDuration := time.Second * 4

	// Case 1: register new segment
	segment0 := fmt.Sprintf("seg-0-%s.ts", uuid.NewString())
	segStart0 := startTime
	segStop0 := segStart0.Add(segDuration)
	var segmentID0 string
	{
		uut := conns.NewPersistanceManager()
		segmentID0, err = uut.RegisterLiveStreamSegment(utCtxt, sourceID, hls.Segment{
			Name:      segment0,
			StartTime: segStart0,
			EndTime:   segStop0,
			Length:    segDuration.Seconds(),
			URI:       fmt.Sprintf("file:///%s", segment0),
		})
		assert.Nil(err)
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
		assert.Nil(seg.Uploaded)
		assert.Equal(fmt.Sprintf("file:///%s", segment0), seg.URI)
		uut.Close()
	}

	// Case 2: register new segment
	segment1 := fmt.Sprintf("seg-1-%s.ts", uuid.NewString())
	segStart1 := segStop0
	segStop1 := segStart1.Add(segDuration)
	var segmentID1 string
	{
		uut := conns.NewPersistanceManager()
		segmentID1, err = uut.RegisterLiveStreamSegment(utCtxt, sourceID, hls.Segment{
			Name:      segment1,
			StartTime: segStart1,
			EndTime:   segStop1,
			Length:    segDuration.Seconds(),
			URI:       fmt.Sprintf("file:///%s", segment1),
		})
		assert.Nil(err)
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
		assert.Nil(seg.Uploaded)
		assert.Equal(fmt.Sprintf("file:///%s", segment1), seg.URI)
		uut.Close()
	}

	// Case 3: mark that a segment is uploaded
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.MarkLiveStreamSegmentsUploaded(utCtxt, []string{segmentID1}))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		seg, err := uut.GetLiveStreamSegment(utCtxt, segmentID1)
		assert.Nil(err)
		assert.NotNil(seg.Uploaded)
		assert.Equal(1, *seg.Uploaded)
		uut.Close()
	}

	// Case 4: fetch segment conditionally
	// By time
	{
		uut := conns.NewPersistanceManager()
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
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
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
		uut.Close()
	}
	// Get latest
	{
		uut := conns.NewPersistanceManager()
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
		uut.Close()
	}

	// Case 5: delete segment
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.DeleteLiveStreamSegment(utCtxt, segmentID1))
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 1)
		seg := entries[0]
		assert.Equal(segment0, seg.Name)
		assert.Equal(segStart0, seg.StartTime)
		assert.Equal(segStop0, seg.EndTime)
		assert.Equal(fmt.Sprintf("file:///%s", segment0), seg.URI)
		uut.Close()
	}

	// Case 6: batch create segments
	var segIDs map[string]string
	{
		uut := conns.NewPersistanceManager()
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
		segIDs, err = uut.BulkRegisterLiveStreamSegments(utCtxt, sourceID, segmentEntries)
		assert.Nil(err)
		assert.Len(segIDs, 2)
		for _, segName := range segments {
			_, ok := segIDs[segName]
			assert.True(ok)
		}
		uut.Close()
	}

	// Case 7: batch delete segments
	{
		uut := conns.NewPersistanceManager()
		deleteIDs := []string{}
		for _, segID := range segIDs {
			deleteIDs = append(deleteIDs, segID)
		}
		assert.Nil(uut.BulkDeleteLiveStreamSegment(utCtxt, deleteIDs))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 1)
		assert.Equal(segmentID0, entries[0].ID)
		uut.Close()
	}

	// Case #: delete video source
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.DeleteVideoSource(utCtxt, sourceID))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entries, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(entries, 0)
		uut.Close()
	}
}

func TestDBManagerVideoSegmentPurgeOldSegments(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)
	conns, err := db.NewSQLConnection(db.GetSqliteDialector(testDB, 20), logger.Info, false)
	assert.Nil(err)

	log.Debugf("Using %s", testDB)

	utCtxt := context.Background()

	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.Ready(utCtxt))
		uut.Close()
	}

	// Create a source
	var sourceID string
	{
		uut := conns.NewPersistanceManager()
		sourceID, err = uut.DefineVideoSource(
			utCtxt, uuid.NewString(), 4, nil, nil,
		)
		assert.Nil(err)
		uut.Close()
	}

	currentTime := time.Now().UTC()
	segmentLength := time.Second * 10

	// Define test segments
	testSegments := []hls.Segment{}
	{
		uut := conns.NewPersistanceManager()
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
		uut.Close()
	}

	// Case 0: read back segments
	{
		uut := conns.NewPersistanceManager()
		segments, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(segments, 6)
		for idx, segment := range segments {
			assert.Equal(testSegments[idx].Name, segment.Name)
		}
		uut.Close()
	}

	// Case 1: delete segments older than time
	{
		uut := conns.NewPersistanceManager()
		markTime := currentTime.Add(segmentLength * 3)
		assert.Nil(uut.DeleteOldLiveStreamSegments(utCtxt, markTime))
		segments, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(segments, 4)
		for idx, segment := range segments {
			assert.Equal(testSegments[idx+2].Name, segment.Name)
		}
		uut.Close()
	}

	// Case 2: delete segments older than time
	{
		uut := conns.NewPersistanceManager()
		markTime := currentTime.Add(segmentLength * 6)
		assert.Nil(uut.DeleteOldLiveStreamSegments(utCtxt, markTime))
		segments, err := uut.ListAllLiveStreamSegments(utCtxt, sourceID)
		assert.Nil(err)
		assert.Len(segments, 1)
		assert.Equal(testSegments[5].Name, segments[0].Name)
		uut.Close()
	}
}

func TestDBManagerVideoRecording(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)
	conns, err := db.NewSQLConnection(db.GetSqliteDialector(testDB, 20), logger.Info, false)
	assert.Nil(err)

	log.Debugf("Using %s", testDB)

	utCtxt := context.Background()

	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.Ready(utCtxt))
		uut.Close()
	}

	// Case 0: no recordings
	{
		uut := conns.NewPersistanceManager()
		_, err := uut.GetRecordingSession(utCtxt, uuid.NewString())
		assert.NotNil(err)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		result, err := uut.ListRecordingSessions(utCtxt)
		assert.Nil(err)
		assert.Len(result, 0)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		result, err := uut.ListRecordingSessionsOfSource(utCtxt, uuid.NewString(), false)
		assert.Nil(err)
		assert.Len(result, 0)
		uut.Close()
	}

	testSource0 := uuid.NewString()
	currentTime := time.Now().UTC()

	// Case 1: define new recording
	recordingID0 := ""
	{
		uut := conns.NewPersistanceManager()
		entryID, err := uut.DefineRecordingSession(utCtxt, testSource0, nil, nil, currentTime)
		assert.Nil(err)
		recordingID0 = entryID
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entry, err := uut.GetRecordingSession(utCtxt, recordingID0)
		assert.Nil(err)
		assert.Equal(recordingID0, entry.ID)
		assert.Equal(testSource0, entry.SourceID)
		assert.Equal(1, entry.Active)
		assert.Equal(currentTime.Unix(), entry.StartTime.Unix())
		uut.Close()
	}

	// Case 2: update recording entry
	{
		uut := conns.NewPersistanceManager()
		testAlias := uuid.NewString()
		testDescription := uuid.NewString()
		assert.Nil(uut.UpdateRecordingSession(utCtxt, common.Recording{
			ID: recordingID0, Alias: &testAlias, Description: &testDescription,
		}))

		entry, err := uut.GetRecordingSession(utCtxt, recordingID0)
		assert.Nil(err)
		assert.Equal(testAlias, *entry.Alias)
		assert.Equal(testDescription, *entry.Description)
		uut.Close()
	}

	// Case 3: create new recording
	recordingID1 := ""
	{
		uut := conns.NewPersistanceManager()
		entryID, err := uut.DefineRecordingSession(utCtxt, testSource0, nil, nil, currentTime)
		assert.Nil(err)
		recordingID1 = entryID
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entries, err := uut.ListRecordingSessions(utCtxt)
		assert.Nil(err)
		assert.Len(entries, 2)
		entryByID := map[string]common.Recording{}
		for _, entry := range entries {
			entryByID[entry.ID] = entry
		}
		assert.Contains(entryByID, recordingID0)
		assert.Contains(entryByID, recordingID1)
		entry := entryByID[recordingID1]
		assert.Equal(recordingID1, entry.ID)
		assert.Equal(testSource0, entry.SourceID)
		assert.Equal(1, entry.Active)
		assert.Equal(currentTime.Unix(), entry.StartTime.Unix())
		uut.Close()
	}

	// Case 4: mark one recording finished
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.MarkEndOfRecordingSession(utCtxt, recordingID1, currentTime))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entries, err := uut.ListRecordingSessionsOfSource(utCtxt, testSource0, false)
		assert.Nil(err)
		assert.Len(entries, 2)
		entryByID := map[string]common.Recording{}
		for _, entry := range entries {
			entryByID[entry.ID] = entry
		}
		assert.Contains(entryByID, recordingID0)
		assert.Contains(entryByID, recordingID1)
		entries, err = uut.ListRecordingSessionsOfSource(utCtxt, testSource0, true)
		assert.Nil(err)
		assert.Len(entries, 1)
		assert.Equal(recordingID0, entries[0].ID)
		uut.Close()
	}

	testSource1 := uuid.NewString()

	// Case 5: create new recording with different source
	recordingID2 := ""
	{
		uut := conns.NewPersistanceManager()
		entryID, err := uut.DefineRecordingSession(utCtxt, testSource1, nil, nil, currentTime)
		assert.Nil(err)
		recordingID2 = entryID
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entries, err := uut.ListRecordingSessions(utCtxt)
		assert.Nil(err)
		assert.Len(entries, 3)
		entries, err = uut.ListRecordingSessionsOfSource(utCtxt, testSource1, false)
		assert.Nil(err)
		assert.Len(entries, 1)
		entry := entries[0]
		assert.Equal(recordingID2, entry.ID)
		assert.Equal(testSource1, entry.SourceID)
		assert.Equal(1, entry.Active)
		assert.Equal(currentTime.Unix(), entry.StartTime.Unix())
		uut.Close()
	}

	var recording1 common.Recording
	{
		uut := conns.NewPersistanceManager()
		recording1, err = uut.GetRecordingSession(utCtxt, recordingID1)
		assert.Nil(err)
		uut.Close()
	}

	// Case 6: delete recording
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.DeleteRecordingSession(utCtxt, recordingID1))
		uut.Close()
	}

	// Case 7: recreate deleted record
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.RecordKnownRecordingSession(utCtxt, recording1))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entry, err := uut.GetRecordingSession(utCtxt, recordingID1)
		assert.Nil(err)
		assert.Equal(recordingID1, entry.ID)
		assert.Equal(testSource0, entry.SourceID)
		assert.Equal(-1, entry.Active)
		assert.Equal(currentTime.Unix(), entry.StartTime.Unix())
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		entries, err := uut.ListRecordingSessionsOfSource(utCtxt, testSource0, true)
		assert.Nil(err)
		assert.Len(entries, 1)
		assert.Equal(recordingID0, entries[0].ID)
		uut.Close()
	}

	// Case 8: query recording by alias
	{
		uut := conns.NewPersistanceManager()
		newAlias := uuid.NewString()
		entry, err := uut.GetRecordingSession(utCtxt, recordingID0)
		assert.Nil(err)
		entry.Alias = &newAlias
		assert.Nil(uut.UpdateRecordingSession(utCtxt, entry))

		entry, err = uut.GetRecordingSessionByAlias(utCtxt, newAlias)
		assert.Nil(err)
		assert.Equal(recordingID0, entry.ID)
		assert.Equal(newAlias, *entry.Alias)
		uut.Close()
	}
}

func getUnitTestPSQLConfig(assert *assert.Assertions) (common.PostgresConfig, string) {
	pgHost := os.Getenv("PGHOST")
	assert.NotEmpty(pgHost)
	pgPortRaw := os.Getenv("PGPORT")
	assert.NotEmpty(pgPortRaw)
	pgPort, err := strconv.Atoi(pgPortRaw)
	assert.Nil(err)
	pgDB := os.Getenv("PGDATABASE")
	assert.NotEmpty(pgDB)
	pgUser := os.Getenv("PGUSER")
	assert.NotEmpty(pgUser)
	pgPassword := os.Getenv("PGPASSWORD")
	assert.NotEmpty(pgPassword)
	return common.PostgresConfig{
		Host: pgHost, Port: uint16(pgPort), Database: pgDB, User: pgUser,
	}, pgPassword
}

func TestDBManagerRecordingSegments(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	sqlDialector, err := db.GetPostgresDialector(getUnitTestPSQLConfig(assert))
	assert.Nil(err)

	conns, err := db.NewSQLConnection(sqlDialector, logger.Info, false)
	assert.Nil(err)

	utCtxt := context.Background()

	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.Ready(utCtxt))
		uut.Close()
	}

	testSourceID := uuid.NewString()

	timestamp := time.Now().UTC()

	// Prepare test sessions
	sessionIDs := []string{}
	{
		uut := conns.NewPersistanceManager()
		for itr := 0; itr < 3; itr++ {
			entryID, err := uut.DefineRecordingSession(utCtxt, testSourceID, nil, nil, timestamp)
			assert.Nil(err)
			sessionIDs = append(sessionIDs, entryID)
		}
		uut.Close()
	}

	// Case 0: create segment associated with unknown recording
	{
		uut := conns.NewPersistanceManager()
		testSegment := common.VideoSegment{
			ID:       ulid.Make().String(),
			SourceID: testSourceID,
			Segment: hls.Segment{
				Name:    uuid.NewString(),
				EndTime: timestamp,
			},
		}
		assert.NotNil(
			uut.RegisterRecordingSegments(
				utCtxt, []string{uuid.NewString()}, []common.VideoSegment{testSegment},
			),
		)
		uut.Close()
	}

	// Create test segments
	testSegments0 := []common.VideoSegment{}
	for itr := 0; itr < 3; itr++ {
		testSegments0 = append(
			testSegments0, common.VideoSegment{
				ID:       ulid.Make().String(),
				SourceID: testSourceID,
				Segment: hls.Segment{
					Name:    uuid.NewString(),
					EndTime: timestamp,
				},
			},
		)
	}

	// Case 1: install segment with recording association
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.RegisterRecordingSegments(utCtxt, []string{sessionIDs[0]}, testSegments0[0:2]))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		segments, err := uut.ListAllSegmentsOfRecording(utCtxt, sessionIDs[0])
		assert.Nil(err)
		assert.Len(segments, 2)
		segByID := map[string]common.VideoSegment{}
		for _, segment := range segments {
			segByID[segment.ID] = segment
		}
		assert.Contains(segByID, testSegments0[0].ID)
		assert.EqualValues(testSegments0[0].Name, segByID[testSegments0[0].ID].Name)
		assert.Contains(segByID, testSegments0[1].ID)
		assert.EqualValues(testSegments0[1].Name, segByID[testSegments0[1].ID].Name)
		uut.Close()
	}

	// Case 2: install the same segments, but with more associations
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.RegisterRecordingSegments(utCtxt, sessionIDs[1:2], testSegments0[0:2]))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		segments, err := uut.ListAllSegmentsOfRecording(utCtxt, sessionIDs[0])
		assert.Nil(err)
		assert.Len(segments, 2)
		segments, err = uut.ListAllSegmentsOfRecording(utCtxt, sessionIDs[1])
		assert.Nil(err)
		assert.Len(segments, 2)
		segByID := map[string]common.VideoSegment{}
		for _, segment := range segments {
			segByID[segment.ID] = segment
		}
		assert.Contains(segByID, testSegments0[0].ID)
		assert.Contains(segByID, testSegments0[1].ID)
		uut.Close()
	}

	// Case 3: install segment with recording association
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.RegisterRecordingSegments(utCtxt, sessionIDs[1:3], testSegments0[2:3]))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		segments, err := uut.ListAllSegmentsOfRecording(utCtxt, sessionIDs[1])
		assert.Nil(err)
		assert.Len(segments, 3)
		segByID := map[string]common.VideoSegment{}
		for _, segment := range segments {
			segByID[segment.ID] = segment
		}
		assert.Contains(segByID, testSegments0[0].ID)
		assert.Contains(segByID, testSegments0[1].ID)
		assert.Contains(segByID, testSegments0[2].ID)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		segments, err := uut.ListAllSegmentsOfRecording(utCtxt, sessionIDs[2])
		assert.Nil(err)
		assert.Len(segments, 1)
		assert.Equal(testSegments0[2].Name, segments[0].Name)
		uut.Close()
	}

	// Case 4: delete recording
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.DeleteRecordingSession(utCtxt, sessionIDs[0]))
		assert.Nil(uut.DeleteRecordingSession(utCtxt, sessionIDs[1]))
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		segments, err := uut.ListAllSegmentsOfRecording(utCtxt, sessionIDs[0])
		assert.Nil(err)
		assert.Len(segments, 0)
		segments, err = uut.ListAllSegmentsOfRecording(utCtxt, sessionIDs[1])
		assert.Nil(err)
		assert.Len(segments, 0)
		segments, err = uut.ListAllSegmentsOfRecording(utCtxt, sessionIDs[2])
		assert.Nil(err)
		assert.Len(segments, 1)
		segments, err = uut.ListAllRecordingSegments(utCtxt)
		assert.Nil(err)
		segByID := map[string]common.VideoSegment{}
		for _, segment := range segments {
			segByID[segment.ID] = segment
		}
		assert.Contains(segByID, testSegments0[0].ID)
		assert.Contains(segByID, testSegments0[1].ID)
		assert.Contains(segByID, testSegments0[2].ID)
		uut.Close()
	}

	// Case 5: purge the segments not associated with any recordings
	{
		uut := conns.NewPersistanceManager()
		deleted, err := uut.DeleteUnassociatedRecordingSegments(utCtxt)
		assert.Nil(err)
		segByID := map[string]common.VideoSegment{}
		for _, segment := range deleted {
			segByID[segment.ID] = segment
		}
		assert.Contains(segByID, testSegments0[0].ID)
		assert.Contains(segByID, testSegments0[1].ID)
		uut.Close()
	}
	{
		uut := conns.NewPersistanceManager()
		segments, err := uut.ListAllRecordingSegments(utCtxt)
		assert.Nil(err)
		segByID := map[string]common.VideoSegment{}
		for _, segment := range segments {
			segByID[segment.ID] = segment
		}
		assert.NotContains(segByID, testSegments0[0].ID)
		assert.NotContains(segByID, testSegments0[1].ID)
		assert.Contains(segByID, testSegments0[2].ID)
		uut.Close()
	}

	// Case 6: clear out all recordings
	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.DeleteRecordingSession(utCtxt, sessionIDs[2]))
		uut.Close()
	}
	// Purge all remaining segments
	{
		uut := conns.NewPersistanceManager()
		deleted, err := uut.DeleteUnassociatedRecordingSegments(utCtxt)
		assert.Nil(err)
		segByID := map[string]common.VideoSegment{}
		for _, segment := range deleted {
			segByID[segment.ID] = segment
		}
		assert.Contains(segByID, testSegments0[2].ID)
		uut.Close()
	}
}

func TestDBManagerParallelTableWries(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)

	testInstance := fmt.Sprintf("ut-%s", uuid.NewString())
	testDB := fmt.Sprintf("/tmp/%s.db", testInstance)
	conns, err := db.NewSQLConnection(db.GetSqliteDialector(testDB, 20), logger.Info, false)
	assert.Nil(err)

	log.Debugf("Using %s", testDB)

	utCtxt := context.Background()

	{
		uut := conns.NewPersistanceManager()
		assert.Nil(uut.Ready(utCtxt))
		uut.Close()
	}

	// Create a source
	var sourceID string
	{
		uut := conns.NewPersistanceManager()
		sourceID, err = uut.DefineVideoSource(utCtxt, uuid.NewString(), 4, nil, nil)
		assert.Nil(err)
		uut.Close()
	}

	startTime := time.Now().UTC()
	segDuration := time.Second * 4

	// Register segments in parallel
	testSegments := 1000
	writers := 10
	segmentPerWriter := int(testSegments / writers)
	segmentIDs := map[int]string{}
	recordSegment := func(writerID int, idx int) {
		segmentIDX := writerID*segmentPerWriter + idx
		segmentName := fmt.Sprintf("seg-%d-%s.ts", segmentIDX, uuid.NewString())
		segStart := startTime.Add(segDuration * time.Duration(segmentIDX))
		segStop := segStart.Add(segDuration)
		uut := conns.NewPersistanceManager()
		segmentID, err := uut.RegisterLiveStreamSegment(utCtxt, sourceID, hls.Segment{
			Name:      segmentName,
			StartTime: segStart,
			EndTime:   segStop,
			Length:    segDuration.Seconds(),
			URI:       fmt.Sprintf("file:///%s", segmentName),
		})
		uut.Close()
		assert.Nil(err)
		segmentIDs[segmentIDX] = segmentID
	}

	// Start the writers
	wg := sync.WaitGroup{}
	wg.Add(writers)
	for itr := 0; itr < writers; itr++ {
		go func() {
			defer wg.Done()
			for segIdx := 0; segIdx < segmentPerWriter; segIdx++ {
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(20)))
				log.WithField("writer", itr).WithField("segment", segIdx).Debug("Recording segment")
				recordSegment(itr, segIdx)
				log.WithField("writer", itr).WithField("segment", segIdx).Debug("Recorded segment")
			}
		}()
	}

	assert.Nil(goutils.TimeBoundedWaitGroupWait(utCtxt, &wg, time.Second*10))
}
