package db

import (
	"context"
	"fmt"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/hls"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PersistenceManager database access layer
type PersistenceManager interface {
	/*
		Ready check whether the DB connection is working

			@param ctxt context.Context - execution context
	*/
	Ready(ctxt context.Context) error

	/*
		Close commit or rollback SQL queries within associated with this instance
	*/
	Close()

	/*
		Error error which occurred during the transaction

			@returns the error
	*/
	Error() error

	/*
		MarkExternalError record an external error with the manager to indicate failure that
		is unrelated to SQL operations but required SQL transaction rollback.

		This would force a transaction rollback when `Close` is called

			@param err error - the external error
	*/
	MarkExternalError(err error)

	// =====================================================================================
	// Video sources

	/*
		DefineVideoSource create new video source

			@param ctxt context.Context - execution context
			@param name string - source name
			@param segmentLen int - target segment length in secs
			@param playlistURI *string - video source playlist URI
			@param description *string - optionally, source description
			@returns new source entry ID
	*/
	DefineVideoSource(
		ctxt context.Context, name string, segmentLen int, playlistURI, description *string,
	) (string, error)

	/*
		RecordKnownVideoSource create record for a known video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
			@param name string - source name
			@param segmentLen int - target segment length in secs
			@param playlistURI *string - video source playlist URI
			@param description *string - optionally, source description
			@param streaming int - whether the video source is currently streaming
	*/
	RecordKnownVideoSource(
		ctxt context.Context,
		id, name string,
		segmentLen int,
		playlistURI, description *string,
		streaming int,
	) error

	/*
		GetVideoSource retrieve a video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
			@returns video source entry
	*/
	GetVideoSource(ctxt context.Context, id string) (common.VideoSource, error)

	/*
		GetVideoSourceByName retrieve a video source by name

			@param ctxt context.Context - execution context
			@param name string - source name
			@returns video source entry
	*/
	GetVideoSourceByName(ctxt context.Context, name string) (common.VideoSource, error)

	/*
		ListVideoSources list all video sources

			@param ctxt context.Context - execution context
			@returns all video source entries
	*/
	ListVideoSources(ctxt context.Context) ([]common.VideoSource, error)

	/*
		UpdateVideoSource update properties of a video source.

		Only the following can be updated:
		  * Name
		  * Description
		  * Playlist URI

			@param ctxt context.Context - execution context
			@param newSetting common.VideoSource - new properties
	*/
	UpdateVideoSource(ctxt context.Context, newSetting common.VideoSource) error

	/*
		ChangeVideoSourceStreamState change the streaming state for a video source

			@param ctxt context.Context - execution context
			@param id string - source ID
			@param streaming int - new streaming state
	*/
	ChangeVideoSourceStreamState(ctxt context.Context, id string, streaming int) error

	/*
		UpdateVideoSourceStats update video source status fields

			@param ctxt context.Context - execution context
			@param id string - source ID
			@param reqRespTargetID string - the request-response target ID for reaching video source
			    over request-response network.
			@param sourceLocalTime time.Time - video source local time
	*/
	UpdateVideoSourceStats(
		ctxt context.Context, id string, reqRespTargetID string, sourceLocalTime time.Time,
	) error

	/*
		DeleteVideoSource delete a video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
	*/
	DeleteVideoSource(ctxt context.Context, id string) error

	// =====================================================================================
	// Live Stream Video segments

	/*
		RegisterLiveStreamSegment record a new segment with a source

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param segment hls.Segment - video segment parameters
			@returns new segment entry ID
	*/
	RegisterLiveStreamSegment(
		ctxt context.Context, sourceID string, segment hls.Segment,
	) (string, error)

	/*
		BulkRegisterLiveStreamSegments register multiple segment in bulk

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param segments []hls.Segment - set of video segments to insert
			@returns new segment entry IDs
	*/
	BulkRegisterLiveStreamSegments(
		ctxt context.Context, sourceID string, segments []hls.Segment,
	) (map[string]string, error)

	/*
		ListAllLiveStreamSegments fetch all video segments

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@returns list of segments
	*/
	ListAllLiveStreamSegments(ctxt context.Context, sourceID string) ([]common.VideoSegment, error)

	/*
		GetLiveStreamSegment get a video segment

			@param ctxt context.Context - execution context
			@param id string - video segment ID
			@returns segment
	*/
	GetLiveStreamSegment(ctxt context.Context, id string) (common.VideoSegment, error)

	/*
		GetLiveStreamSegmentByName get a video segment by name

			@param ctxt context.Context - execution context
			@param name string - video segment name
			@returns segment
	*/
	GetLiveStreamSegmentByName(ctxt context.Context, name string) (common.VideoSegment, error)

	/*
		ListAllLiveStreamSegmentsAfterTime fetch all video segments which have a stop timestamp
		after a timestamp

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param timestamp time.Time - timestamp to check against
			@returns list of segments
	*/
	ListAllLiveStreamSegmentsAfterTime(
		ctxt context.Context, sourceID string, timestamp time.Time,
	) ([]common.VideoSegment, error)

	/*
		GetLatestLiveStreamSegments get the latest N video segments

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param count int - number of segments to get
			@returns list of segments
	*/
	GetLatestLiveStreamSegments(
		ctxt context.Context, sourceID string, count int,
	) ([]common.VideoSegment, error)

	/*
		MarkLiveStreamSegmentsUploaded mark a set of live segment as uploaded

			@param ctxt context.Context - execution context
			@param ids []string - segment IDs
	*/
	MarkLiveStreamSegmentsUploaded(ctxt context.Context, ids []string) error

	/*
		DeleteLiveStreamSegment delete a segment

			@param ctxt context.Context - execution context
			@param id string - video segment ID
	*/
	DeleteLiveStreamSegment(ctxt context.Context, id string) error

	/*
		BulkDeleteLiveStreamSegment delete a group of segments

			@param ctxt context.Context - execution context
			@param ids []string - video segment IDs
	*/
	BulkDeleteLiveStreamSegment(ctxt context.Context, ids []string) error

	/*
		PurgeOldSegments delete segments older than a specific point in time

			@param ctxt context.Context - execution context
			@param timeLimit time.Time - video segment older than this point will be purged
	*/
	DeleteOldLiveStreamSegments(
		ctxt context.Context, timeLimit time.Time,
	) error

	// =====================================================================================
	// Video Recording Sessions

	/*
		DefineRecordingSession create new video recording session

			@param ctxt context.Context - execution context
			@param sourceID string - the video source ID
			@param alias *string - an optional alias name for the recording session
			@param description *string - an optional description of the recording session
			@param startTime time.Time - when the recording session started
			@returns new recording session ID
	*/
	DefineRecordingSession(
		ctxt context.Context, sourceID string, alias, description *string, startTime time.Time,
	) (string, error)

	/*
		RecordKnownRecordingSession create record for an existing video recording session

			@param ctxt context.Context - execution context
			@param entry common.Recording - video recording session entry
	*/
	RecordKnownRecordingSession(ctxt context.Context, entry common.Recording) error

	/*
		GetRecordingSession retrieve a video recording session

			@param ctxt context.Context - execution context
			@param id string - session entry ID
			@returns video recording entry
	*/
	GetRecordingSession(ctxt context.Context, id string) (common.Recording, error)

	/*
		GetRecordingSessionByAlias retrieve a video recording session by alias

			@param ctxt context.Context - execution context
			@param alias string - session entry alias
			@returns video recording entry
	*/
	GetRecordingSessionByAlias(ctxt context.Context, alias string) (common.Recording, error)

	/*
		ListRecordingSessions list all video recording sessions

			@param ctxt context.Context - execution context
			@returns all recording sessions
	*/
	ListRecordingSessions(ctxt context.Context) ([]common.Recording, error)

	/*
		ListRecordingSessionsOfSource list all video recording sessions of a video source

			@param ctxt context.Context - execution context
			@param sourceID string - the video source ID
			@param active bool - if 1, select only the active recording sessions; else return all.
			@returns all recording sessions of a video source
	*/
	ListRecordingSessionsOfSource(
		ctxt context.Context, sourceID string, active bool,
	) ([]common.Recording, error)

	/*
		MarkEndOfRecordingSession mark a video recording session as complete.

			@param ctxt context.Context - execution context
			@param id string - session entry ID
			@param endTime time.Time - when the recording session ended
	*/
	MarkEndOfRecordingSession(ctxt context.Context, id string, endTime time.Time) error

	/*
		UpdateRecordingSession update properties of a video recording session.

		Only the following can be updated:
		  * Alias
		  * Description

			@param ctxt context.Context - execution context
			@param newSetting common.Recording - new properties
	*/
	UpdateRecordingSession(ctxt context.Context, newSetting common.Recording) error

	/*
		DeleteRecordingSession delete a video recording session

			@param ctxt context.Context - execution context
			@param id string - session entry ID
	*/
	DeleteRecordingSession(ctxt context.Context, id string) error

	// =====================================================================================
	// Recording Session Video segments

	/*
		RegisterRecordingSegments record a set of new segments with a set of video recording sessions

			@param ctxt context.Context - execution context
			@param recordingIDs []string - video recording session IDs
			@param segments []common.VideoSegment - video segments
	*/
	RegisterRecordingSegments(
		ctxt context.Context, recordingIDs []string, segments []common.VideoSegment,
	) error

	/*
		GetRecordingSegments get a video segment of a video recording session

			@param ctxt context.Context - execution context
			@param id string - segment ID
			@returns the video segment
	*/
	GetRecordingSegment(ctxt context.Context, id string) (common.VideoSegment, error)

	/*
		GetRecordingSegmentByName get a video segment of a video recording session by name

			@param ctxt context.Context - execution context
			@param name string - video segment name
			@returns the video segment
	*/
	GetRecordingSegmentByName(ctxt context.Context, name string) (common.VideoSegment, error)

	/*
		ListAllRecordingSegments fetch all video segments belonging to recordings

			@param ctxt context.Context - execution context
			@returns set of video segments
	*/
	ListAllRecordingSegments(ctxt context.Context) ([]common.VideoSegment, error)

	/*
		ListAllSegmentsOfRecording fetch all video segments belonging to one recording session

			@param ctxt context.Context - execution context
			@param recordingID string - video recording session ID
			@returns set of video segments
	*/
	ListAllSegmentsOfRecording(
		ctxt context.Context, recordingID string,
	) ([]common.VideoSegment, error)

	/*
		DeleteUnassociatedRecordingSegments delete recording segments not attached to any
		video recording sessions

			@param ctxt context.Context - execution context
			@returns set of segments deleted
	*/
	DeleteUnassociatedRecordingSegments(ctxt context.Context) ([]common.VideoSegment, error)
}

// persistenceManagerImpl implements PersistenceManager
type persistenceManagerImpl struct {
	goutils.Component
	conn      ConnectionManager
	db        *gorm.DB
	validator *validator.Validate
	err       error
}

/*
newManager define a new DB access manager

	@param dbDialector gorm.Dialector - GORM SQL dialector
	@param logLevel logger.LogLevel - SQL log level
	@returns new manager
*/
func newManager(connections ConnectionManager) PersistenceManager {
	logTags := log.Fields{"module": "db", "component": "manager"}
	return &persistenceManagerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		conn:      connections,
		db:        connections.NewTransaction(),
		validator: validator.New(),
		err:       nil,
	}
}

func (m *persistenceManagerImpl) Ready(ctxt context.Context) error {
	return m.db.Find(&[]videoSource{}).Limit(1).Error
}

func (m *persistenceManagerImpl) Close() {
	if m.err != nil {
		m.conn.Rollback(m.db)
	} else {
		m.conn.Commit(m.db)
	}
}

func (m *persistenceManagerImpl) Error() error {
	return m.err
}

func (m *persistenceManagerImpl) MarkExternalError(err error) {
	m.err = err
}

// =====================================================================================
// Video sources

func (m *persistenceManagerImpl) DefineVideoSource(
	ctxt context.Context, name string, segmentLen int, playlistURI, description *string,
) (string, error) {
	logTags := m.GetLogTagsForContext(ctxt)

	// Do not continue if class instance already logged an error
	if m.err != nil {
		return "", fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	// Prepare new entry
	newEntryID := uuid.NewString()
	newEntry := videoSource{
		VideoSource: common.VideoSource{
			ID:                  newEntryID,
			Name:                name,
			TargetSegmentLength: segmentLen,
			Description:         description,
			PlaylistURI:         playlistURI,
			Streaming:           -1,
		},
	}

	// Verify data
	if err := m.validator.Struct(&newEntry); err != nil {
		m.err = err
		return newEntryID, err
	}

	// Insert entry
	if tmp := m.db.Create(&newEntry); tmp.Error != nil {
		m.err = tmp.Error
		return newEntryID, tmp.Error
	}

	log.
		WithFields(logTags).
		WithField("name", name).
		WithField("playlist", playlistURI).
		WithField("id", newEntryID).
		Info("Defined new video source")

	return newEntryID, nil
}

func (m *persistenceManagerImpl) RecordKnownVideoSource(
	ctxt context.Context,
	id, name string,
	segmentLen int,
	playlistURI, description *string,
	streaming int,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	// Prepare entry
	entry := videoSource{
		VideoSource: common.VideoSource{
			ID:                  id,
			Name:                name,
			TargetSegmentLength: segmentLen,
			Description:         description,
			PlaylistURI:         playlistURI,
			Streaming:           streaming,
		},
	}

	// Insert entry
	if tmp := m.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&entry); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}

	log.
		WithFields(logTags).
		WithField("name", name).
		WithField("playlist", playlistURI).
		WithField("id", id).
		Info("Recorded known video source")
	return nil
}

func (m *persistenceManagerImpl) GetVideoSource(
	ctxt context.Context, id string,
) (common.VideoSource, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return common.VideoSource{},
			fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var entry videoSource
	if tmp := m.db.First(&entry, "id = ?", id); tmp.Error != nil {
		m.err = tmp.Error
		return common.VideoSource{}, tmp.Error
	}
	result := entry.VideoSource
	return result, nil
}

func (m *persistenceManagerImpl) GetVideoSourceByName(
	ctxt context.Context, name string,
) (common.VideoSource, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return common.VideoSource{},
			fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var entry videoSource
	if tmp := m.db.First(&entry, "name = ?", name); tmp.Error != nil {
		m.err = tmp.Error
		return common.VideoSource{}, tmp.Error
	}
	result := entry.VideoSource
	return result, nil
}

func (m *persistenceManagerImpl) ListVideoSources(ctxt context.Context) ([]common.VideoSource, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var result []common.VideoSource
	var entries []videoSource
	if tmp := m.db.Find(&entries); tmp.Error != nil {
		m.err = tmp.Error
		return nil, tmp.Error
	}
	for _, entry := range entries {
		result = append(result, entry.VideoSource)
	}
	return result, nil
}

func (m *persistenceManagerImpl) UpdateVideoSource(
	ctxt context.Context, newSetting common.VideoSource,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	logTags := m.GetLogTagsForContext(ctxt)
	if tmp := m.db.Where(&videoSource{VideoSource: common.VideoSource{
		ID: newSetting.ID,
	}}).Updates(&videoSource{VideoSource: common.VideoSource{
		Name:        newSetting.Name,
		Description: newSetting.Description,
		PlaylistURI: newSetting.PlaylistURI,
	}}); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}

	log.
		WithFields(logTags).
		WithField("name", newSetting.Name).
		WithField("playlist", newSetting.PlaylistURI).
		WithField("id", newSetting.ID).
		Info("Updated video source")

	return nil
}

func (m *persistenceManagerImpl) ChangeVideoSourceStreamState(
	ctxt context.Context, id string, streaming int,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	if tmp := m.db.Where(&videoSource{VideoSource: common.VideoSource{
		ID: id,
	}}).Updates(&videoSource{VideoSource: common.VideoSource{
		Streaming: streaming,
	}}); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	return nil
}

func (m *persistenceManagerImpl) UpdateVideoSourceStats(
	ctxt context.Context, id string, reqRespTargetID string, sourceLocalTime time.Time,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	if tmp := m.db.Where(&videoSource{VideoSource: common.VideoSource{
		ID: id,
	}}).Updates(&videoSource{VideoSource: common.VideoSource{
		ReqRespTargetID: &reqRespTargetID,
		SourceLocalTime: sourceLocalTime,
	}}); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	return nil
}

func (m *persistenceManagerImpl) DeleteVideoSource(ctxt context.Context, id string) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	logTags := m.GetLogTagsForContext(ctxt)
	if tmp := m.db.Where(
		&liveStreamVideoSegment{VideoSegment: common.VideoSegment{SourceID: id}},
	).Delete(&liveStreamVideoSegment{}); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	if tmp := m.db.Delete(&videoSource{VideoSource: common.VideoSource{ID: id}}); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	log.WithFields(logTags).WithField("id", id).Info("Delete video source")
	return nil
}

// =====================================================================================
// Live Stream Video segments

func (m *persistenceManagerImpl) RegisterLiveStreamSegment(
	ctxt context.Context, sourceID string, segment hls.Segment,
) (string, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return "", fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	logTags := m.GetLogTagsForContext(ctxt)

	newID := ulid.Make().String()

	newEntry := liveStreamVideoSegment{
		VideoSegment: common.VideoSegment{
			ID:       newID,
			Segment:  segment,
			SourceID: sourceID,
		},
	}

	// Verify data
	if err := m.validator.Struct(&newEntry); err != nil {
		m.err = err
		return "", err
	}

	// Insert entry
	if tmp := m.db.Create(&newEntry); tmp.Error != nil {
		m.err = tmp.Error
		return "", tmp.Error
	}

	log.
		WithFields(logTags).
		WithField("source-id", sourceID).
		WithField("id", newID).
		WithField("segment", segment.String()).
		Debug("Registered new video segment")
	return newID, nil
}

func (m *persistenceManagerImpl) BulkRegisterLiveStreamSegments(
	ctxt context.Context, sourceID string, segments []hls.Segment,
) (map[string]string, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	newIDs := map[string]string{}
	newEntries := []liveStreamVideoSegment{}
	// Prepare entries for batch insert
	for _, segment := range segments {
		newID := ulid.Make().String()
		newEntry := liveStreamVideoSegment{
			VideoSegment: common.VideoSegment{
				ID:       newID,
				Segment:  segment,
				SourceID: sourceID,
			},
		}
		// Verify data
		if err := m.validator.Struct(&newEntry); err != nil {
			m.err = err
			return nil, err
		}
		newEntries = append(newEntries, newEntry)
		newIDs[segment.Name] = newID
	}

	// Insert entries
	if tmp := m.db.Create(&newEntries); tmp.Error != nil {
		m.err = tmp.Error
		return nil, tmp.Error
	}
	return newIDs, nil
}

func (m *persistenceManagerImpl) ListAllLiveStreamSegments(ctxt context.Context, sourceID string) (
	[]common.VideoSegment, error,
) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var results []common.VideoSegment
	var entries []liveStreamVideoSegment
	if tmp := m.db.Where(
		&liveStreamVideoSegment{VideoSegment: common.VideoSegment{SourceID: sourceID}},
	).Order("end_ts").Find(&entries); tmp.Error != nil {
		m.err = tmp.Error
		return nil, tmp.Error
	}
	for _, entry := range entries {
		results = append(results, entry.VideoSegment)
	}
	return results, nil
}

func (m *persistenceManagerImpl) GetLiveStreamSegment(
	ctxt context.Context, id string,
) (common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return common.VideoSegment{},
			fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var entry liveStreamVideoSegment
	if tmp := m.db.Where(
		&liveStreamVideoSegment{VideoSegment: common.VideoSegment{ID: id}},
	).First(&entry); tmp.Error != nil {
		m.err = tmp.Error
		return common.VideoSegment{}, tmp.Error
	}
	result := entry.VideoSegment
	return result, nil
}

func (m *persistenceManagerImpl) GetLiveStreamSegmentByName(
	ctxt context.Context, name string,
) (common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return common.VideoSegment{},
			fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var entry liveStreamVideoSegment
	if tmp := m.db.Where(
		&liveStreamVideoSegment{VideoSegment: common.VideoSegment{Segment: hls.Segment{Name: name}}},
	).First(&entry); tmp.Error != nil {
		m.err = tmp.Error
		return common.VideoSegment{}, tmp.Error
	}
	result := entry.VideoSegment
	return result, nil
}

func (m *persistenceManagerImpl) ListAllLiveStreamSegmentsAfterTime(
	ctxt context.Context, sourceID string, timestamp time.Time,
) ([]common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var results []common.VideoSegment
	var entries []liveStreamVideoSegment
	if tmp := m.db.
		Where(&liveStreamVideoSegment{VideoSegment: common.VideoSegment{SourceID: sourceID}}).
		Where("end_ts >= ?", timestamp).
		Order("end_ts").
		Find(&entries); tmp.Error != nil {
		m.err = tmp.Error
		return nil, tmp.Error
	}
	for _, entry := range entries {
		results = append(results, entry.VideoSegment)
	}
	return results, nil
}

func (m *persistenceManagerImpl) GetLatestLiveStreamSegments(
	ctxt context.Context, sourceID string, count int,
) ([]common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var results []common.VideoSegment
	var entries []liveStreamVideoSegment
	if tmp := m.db.
		Where(&liveStreamVideoSegment{VideoSegment: common.VideoSegment{SourceID: sourceID}}).
		Order("end_ts desc").
		Limit(count).
		Find(&entries); tmp.Error != nil {
		m.err = tmp.Error
		return nil, tmp.Error
	}
	results = make([]common.VideoSegment, len(entries))
	for idx, entry := range entries {
		results[len(entries)-idx-1] = entry.VideoSegment
	}
	return results, nil
}

func (m *persistenceManagerImpl) MarkLiveStreamSegmentsUploaded(
	ctxt context.Context, ids []string,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	val := 1
	tmp := m.db.Where("id in ?", ids).Updates(
		&liveStreamVideoSegment{VideoSegment: common.VideoSegment{Uploaded: &val}},
	)
	if tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	return nil
}

func (m *persistenceManagerImpl) DeleteLiveStreamSegment(ctxt context.Context, id string) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	if tmp := m.db.Where(
		&liveStreamVideoSegment{VideoSegment: common.VideoSegment{ID: id}},
	).Delete(&liveStreamVideoSegment{}); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	return nil
}

func (m *persistenceManagerImpl) BulkDeleteLiveStreamSegment(
	ctxt context.Context, ids []string,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	if tmp := m.db.Where("id in ?", ids).Delete(&liveStreamVideoSegment{}); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	return nil
}

func (m *persistenceManagerImpl) DeleteOldLiveStreamSegments(
	ctxt context.Context, timeLimit time.Time,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	if tmp := m.db.
		Where("end_ts < ?", timeLimit).
		Delete(&liveStreamVideoSegment{}); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	return nil
}

// =====================================================================================
// Video Recording Sessions

func (m *persistenceManagerImpl) DefineRecordingSession(
	ctxt context.Context, sourceID string, alias, description *string, startTime time.Time,
) (string, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return "", fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	logTags := m.GetLogTagsForContext(ctxt)

	// Prepare new entry
	newEntryID := ulid.Make().String()

	newEntry := recordingSession{
		Recording: common.Recording{
			ID:          newEntryID,
			Alias:       alias,
			Description: description,
			SourceID:    sourceID,
			StartTime:   startTime,
			Active:      1,
		},
	}

	// Verify data
	if err := m.validator.Struct(&newEntry); err != nil {
		m.err = err
		return "", err
	}

	// Insert entry
	if tmp := m.db.Create(&newEntry); tmp.Error != nil {
		m.err = tmp.Error
		return "", tmp.Error
	}

	log.
		WithFields(logTags).
		WithField("source-id", sourceID).
		WithField("id", newEntryID).
		Info("Registered new video recording session")
	return newEntryID, nil
}

func (m *persistenceManagerImpl) RecordKnownRecordingSession(
	ctxt context.Context, entry common.Recording,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	logTags := m.GetLogTagsForContext(ctxt)

	newEntry := recordingSession{
		Recording: common.Recording{
			ID:          entry.ID,
			Alias:       entry.Alias,
			Description: entry.Description,
			SourceID:    entry.SourceID,
			StartTime:   entry.StartTime,
			EndTime:     entry.EndTime,
			Active:      entry.Active,
		},
	}

	// Verify data
	if err := m.validator.Struct(&newEntry); err != nil {
		m.err = err
		return err
	}

	// Insert entry
	if tmp := m.db.Create(&newEntry); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}

	log.
		WithFields(logTags).
		WithField("source-id", entry.SourceID).
		WithField("id", entry.ID).
		Info("Registered pre-existing video recording session")

	return nil
}

func (m *persistenceManagerImpl) GetRecordingSession(
	ctxt context.Context, id string,
) (common.Recording, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return common.Recording{},
			fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var entry recordingSession

	tmp := m.db.Where(&recordingSession{Recording: common.Recording{ID: id}}).First(&entry)
	if tmp.Error != nil {
		m.err = tmp.Error
		return common.Recording{}, tmp.Error
	}

	result := entry.Recording
	return result, nil
}

func (m *persistenceManagerImpl) GetRecordingSessionByAlias(
	ctxt context.Context, alias string,
) (common.Recording, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return common.Recording{},
			fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var entry recordingSession
	tmp := m.db.Where(&recordingSession{Recording: common.Recording{Alias: &alias}}).First(&entry)
	if tmp.Error != nil {
		m.err = tmp.Error
		return common.Recording{}, tmp.Error
	}

	result := entry.Recording
	return result, nil
}

func (m *persistenceManagerImpl) ListRecordingSessions(
	ctxt context.Context,
) ([]common.Recording, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var results []common.Recording
	var entries []recordingSession

	if tmp := m.db.Order("start_ts").Find(&entries); tmp.Error != nil {
		m.err = tmp.Error
		return nil, tmp.Error
	}

	for _, entry := range entries {
		results = append(results, entry.Recording)
	}
	return results, nil
}

func (m *persistenceManagerImpl) ListRecordingSessionsOfSource(
	ctxt context.Context, sourceID string, active bool,
) ([]common.Recording, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var results []common.Recording
	var entries []recordingSession

	query := m.db.Where(&recordingSession{Recording: common.Recording{SourceID: sourceID}})
	if active {
		query = query.Where(&recordingSession{Recording: common.Recording{Active: 1}})
	}
	if tmp := query.Order("start_ts").Find(&entries); tmp.Error != nil {
		m.err = tmp.Error
		return nil, tmp.Error
	}

	for _, entry := range entries {
		results = append(results, entry.Recording)
	}
	return results, nil
}

func (m *persistenceManagerImpl) MarkEndOfRecordingSession(
	ctxt context.Context, id string, endTime time.Time,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	logTags := m.GetLogTagsForContext(ctxt)

	if tmp := m.db.Where(
		&recordingSession{Recording: common.Recording{ID: id}},
	).Updates(
		&recordingSession{Recording: common.Recording{Active: -1, EndTime: endTime}},
	); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}

	log.WithFields(logTags).WithField("recording", id).Info("Marking recording as complete")

	return nil
}

func (m *persistenceManagerImpl) UpdateRecordingSession(
	ctxt context.Context, newSetting common.Recording,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	if tmp := m.db.Where(
		&recordingSession{Recording: common.Recording{ID: newSetting.ID}},
	).Updates(
		&recordingSession{Recording: common.Recording{
			Alias: newSetting.Alias, Description: newSetting.Description,
		}},
	); tmp.Error != nil {
		m.err = tmp.Error
		return tmp.Error
	}
	return nil
}

func (m *persistenceManagerImpl) DeleteRecordingSession(ctxt context.Context, id string) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	logTags := m.GetLogTagsForContext(ctxt)

	// Get the recording entry first
	var recording recordingSession
	tmp := m.db.Where(
		&recordingSession{Recording: common.Recording{ID: id}},
	).First(&recording)
	if tmp.Error != nil {
		log.
			WithError(tmp.Error).
			WithFields(logTags).
			WithField("recording", id).
			Error("Unable to read video recording session")
		m.err = tmp.Error
		return tmp.Error
	}

	// Delete the association between this session and segments
	tmp = m.db.
		Where(&segmentToRecordingAssociation{RecordingID: id}).
		Delete(&segmentToRecordingAssociation{})
	if tmp.Error != nil {
		log.
			WithError(tmp.Error).
			WithFields(logTags).
			WithField("recording", id).
			Error("Unable to delete associations between recording session and its segments")
		m.err = tmp.Error
		return tmp.Error
	}

	// Delete recording
	if tmp = m.db.Delete(&recording); tmp.Error != nil {
		log.
			WithError(tmp.Error).
			WithFields(logTags).
			WithField("recording", id).
			Error("Unable to delete video recording session")
		m.err = tmp.Error
		return tmp.Error
	}

	log.WithFields(logTags).WithField("recording", id).Info("Deleted recording session")

	return nil
}

// =====================================================================================
// Recording Session Video segments

func (m *persistenceManagerImpl) RegisterRecordingSegments(
	ctxt context.Context, recordingIDs []string, segments []common.VideoSegment,
) error {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	logTags := m.GetLogTagsForContext(ctxt)

	// Fetch the recording sessions first
	var recordings []recordingSession
	if tmp := m.db.Where("id in ?", recordingIDs).Find(&recordings); tmp.Error != nil {
		log.WithError(tmp.Error).WithFields(logTags).Error("Unable to find associated recordings")
		m.err = tmp.Error
		return tmp.Error
	}

	// Form the segment entries
	entries := []recordingVideoSegment{}
	for _, oneSegment := range segments {
		entries = append(entries, recordingVideoSegment{
			VideoSegment: common.VideoSegment{
				ID:       oneSegment.ID,
				SourceID: oneSegment.SourceID,
				Segment:  oneSegment.Segment,
			},
		})
	}

	// Form the segment to recording session associations
	associations := []segmentToRecordingAssociation{}
	for _, oneSegment := range segments {
		for _, recordingID := range recordingIDs {
			associations = append(associations, segmentToRecordingAssociation{
				SegmentID: oneSegment.ID, RecordingID: recordingID,
			})
		}
	}

	// Install the segments first
	if tmp := m.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entries); tmp.Error != nil {
		log.WithError(tmp.Error).WithFields(logTags).Error("Failed to create recording segments")
		m.err = tmp.Error
		return tmp.Error
	}

	// Install the associations
	tmp := m.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&associations)
	if tmp.Error != nil {
		log.
			WithError(tmp.Error).
			WithFields(logTags).
			Error("Failed to create associations between recording sessions and its segments")
		m.err = tmp.Error
		return tmp.Error
	}

	return nil
}

func (m *persistenceManagerImpl) GetRecordingSegment(
	ctxt context.Context, id string,
) (common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return common.VideoSegment{},
			fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var entry recordingVideoSegment
	if tmp := m.db.Where(
		&recordingVideoSegment{VideoSegment: common.VideoSegment{ID: id}},
	).First(&entry); tmp.Error != nil {
		m.err = tmp.Error
		return common.VideoSegment{}, tmp.Error
	}
	result := entry.VideoSegment
	return result, nil
}

func (m *persistenceManagerImpl) GetRecordingSegmentByName(
	ctxt context.Context, name string,
) (common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return common.VideoSegment{},
			fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var entry recordingVideoSegment
	if tmp := m.db.Where(
		&recordingVideoSegment{VideoSegment: common.VideoSegment{Segment: hls.Segment{Name: name}}},
	).First(&entry); tmp.Error != nil {
		m.err = tmp.Error
		return common.VideoSegment{}, tmp.Error
	}
	result := entry.VideoSegment
	return result, nil
}

func (m *persistenceManagerImpl) ListAllRecordingSegments(
	ctxt context.Context,
) ([]common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var result []common.VideoSegment
	var entries []recordingVideoSegment
	if tmp := m.db.Order("end_ts").Find(&entries); tmp.Error != nil {
		m.err = tmp.Error
		return nil, tmp.Error
	}
	for _, entry := range entries {
		result = append(result, entry.VideoSegment)
	}
	return result, nil
}

func (m *persistenceManagerImpl) ListAllSegmentsOfRecording(
	ctxt context.Context, recordingID string,
) ([]common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var result []common.VideoSegment
	logTags := m.GetLogTagsForContext(ctxt)

	// Get the associations first
	var associations []segmentToRecordingAssociation
	tmp := m.db.Where(&segmentToRecordingAssociation{RecordingID: recordingID}).Find(&associations)
	if tmp.Error != nil {
		log.
			WithError(tmp.Error).
			WithFields(logTags).
			WithField("recording", recordingID).
			Error("Unable to select segments associated with video recording session")
		m.err = tmp.Error
		return nil, tmp.Error
	}

	if len(associations) > 0 {
		segIDs := []string{}
		for _, entry := range associations {
			segIDs = append(segIDs, entry.SegmentID)
		}

		// Get the associated segments
		var entries []recordingVideoSegment
		if tmp := m.db.Where("id in ?", segIDs).Order("end_ts").Find(&entries); tmp.Error != nil {
			m.err = tmp.Error
			return nil, tmp.Error
		}
		for _, entry := range entries {
			result = append(result, entry.VideoSegment)
		}
	}
	return result, nil
}

func (m *persistenceManagerImpl) DeleteUnassociatedRecordingSegments(
	ctxt context.Context,
) ([]common.VideoSegment, error) {
	// Do not continue if class instance already logged an error
	if m.err != nil {
		return nil, fmt.Errorf("sql operation can't continue due to existing error '%s'", m.err.Error())
	}

	var result []common.VideoSegment
	logTags := m.GetLogTagsForContext(ctxt)

	// Get the set of distinct segments IDs still with association
	var associations []segmentToRecordingAssociation
	tmp := m.db.Distinct("segment_id").Find(&associations)
	if tmp.Error != nil {
		log.
			WithError(tmp.Error).
			WithFields(logTags).
			Error("Failed to read distinct segment IDs from 'segment_to_recording_association'")
		m.err = tmp.Error
		return nil, tmp.Error
	}

	if len(associations) > 0 {
		// Read and delete the segments which are not mentioned in association table
		segIDs := []string{}
		for _, entry := range associations {
			segIDs = append(segIDs, entry.SegmentID)
		}

		var entries []recordingVideoSegment
		tmp = m.db.Clauses(clause.Returning{}).Not("id in ?", segIDs).Delete(&entries)
		if tmp.Error != nil {
			log.
				WithError(tmp.Error).
				WithFields(logTags).
				Error("Failed to delete segments not mentioned in 'segment_to_recording_association'")
			m.err = tmp.Error
			return nil, tmp.Error
		}

		for _, entry := range entries {
			result = append(result, entry.VideoSegment)
		}
	}
	return result, nil
}
