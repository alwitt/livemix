package db_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/db"
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
