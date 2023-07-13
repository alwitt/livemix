package db

import (
	"context"
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
	"gorm.io/gorm/logger"
)

// PersistenceManager database access layer
type PersistenceManager interface {
	/*
		Ready check whether the DB connection is working

			@param ctxt context.Context - execution context
	*/
	Ready(ctxt context.Context) error

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
		RefreshVideoSourceStats update video source status fields

			@param ctxt context.Context - execution context
			@param id string - source ID
			@param reqRespTargetID string - the request-response target ID for reaching video source
			    over request-response network.
			@param sourceLocalTime time.Time - video source local time
	*/
	RefreshVideoSourceStats(
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
	PurgeOldLiveStreamSegments(
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
			@returns all recording sessions of a video source source
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
}

// persistenceManagerImpl implements PersistenceManager
type persistenceManagerImpl struct {
	goutils.Component
	db        *gorm.DB
	validator *validator.Validate
}

/*
NewManager define a new DB access manager

	@param dbDialector gorm.Dialector - GORM SQL dialector
	@param logLevel logger.LogLevel - SQL log level
	@returns new manager
*/
func NewManager(dbDialector gorm.Dialector, logLevel logger.LogLevel) (PersistenceManager, error) {
	db, err := gorm.Open(dbDialector, &gorm.Config{
		Logger:                 logger.Default.LogMode(logLevel),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	// Prepare the databases
	if err := db.AutoMigrate(&videoSource{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&liveStreamVideoSegment{}); err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&recordingSession{}, &recordingVideoSegment{}); err != nil {
		return nil, err
	}

	logTags := log.Fields{"module": "db", "component": "manager", "instance": dbDialector.Name()}
	return &persistenceManagerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		db:        db,
		validator: validator.New(),
	}, nil
}

func (m *persistenceManagerImpl) Ready(ctxt context.Context) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		tmp := tx.Find(&[]videoSource{}).Limit(1)
		return tmp.Error
	})
}

// =====================================================================================
// Video sources

func (m *persistenceManagerImpl) DefineVideoSource(
	ctxt context.Context, name string, segmentLen int, playlistURI, description *string,
) (string, error) {
	newEntryID := ""
	return newEntryID, m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

		// Prepare new entry
		newEntryID = uuid.NewString()
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
			return err
		}

		// Insert entry
		if tmp := tx.Create(&newEntry); tmp.Error != nil {
			return tmp.Error
		}

		log.
			WithFields(logTags).
			WithField("name", name).
			WithField("playlist", playlistURI).
			WithField("id", newEntryID).
			Info("Defined new video source")
		return nil
	})
}

func (m *persistenceManagerImpl) RecordKnownVideoSource(
	ctxt context.Context,
	id, name string,
	segmentLen int,
	playlistURI, description *string,
	streaming int,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

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
		if tmp := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).Create(&entry); tmp.Error != nil {
			return tmp.Error
		}

		log.
			WithFields(logTags).
			WithField("name", name).
			WithField("playlist", playlistURI).
			WithField("id", id).
			Info("Recorded known video source")
		return nil
	})
}

func (m *persistenceManagerImpl) GetVideoSource(
	ctxt context.Context, id string,
) (common.VideoSource, error) {
	var result common.VideoSource
	return result, m.db.Transaction(func(tx *gorm.DB) error {
		var entry videoSource
		if tmp := tx.First(&entry, "id = ?", id); tmp.Error != nil {
			return tmp.Error
		}
		result = entry.VideoSource
		return nil
	})
}

func (m *persistenceManagerImpl) GetVideoSourceByName(
	ctxt context.Context, name string,
) (common.VideoSource, error) {
	var result common.VideoSource
	return result, m.db.Transaction(func(tx *gorm.DB) error {
		var entry videoSource
		if tmp := tx.First(&entry, "name = ?", name); tmp.Error != nil {
			return tmp.Error
		}
		result = entry.VideoSource
		return nil
	})
}

func (m *persistenceManagerImpl) ListVideoSources(ctxt context.Context) ([]common.VideoSource, error) {
	var result []common.VideoSource
	return result, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []videoSource
		if tmp := tx.Find(&entries); tmp.Error != nil {
			return tmp.Error
		}
		for _, entry := range entries {
			result = append(result, entry.VideoSource)
		}
		return nil
	})
}

func (m *persistenceManagerImpl) UpdateVideoSource(
	ctxt context.Context, newSetting common.VideoSource,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)
		if tmp := tx.Where(&videoSource{VideoSource: common.VideoSource{
			ID: newSetting.ID,
		}}).Updates(&videoSource{VideoSource: common.VideoSource{
			Name:        newSetting.Name,
			Description: newSetting.Description,
			PlaylistURI: newSetting.PlaylistURI,
		}}); tmp.Error != nil {
			return tmp.Error
		}

		log.
			WithFields(logTags).
			WithField("name", newSetting.Name).
			WithField("playlist", newSetting.PlaylistURI).
			WithField("id", newSetting.ID).
			Info("Updated video source")

		return nil
	})
}

func (m *persistenceManagerImpl) ChangeVideoSourceStreamState(
	ctxt context.Context, id string, streaming int,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.Where(&videoSource{VideoSource: common.VideoSource{
			ID: id,
		}}).Updates(&videoSource{VideoSource: common.VideoSource{
			Streaming: streaming,
		}}); tmp.Error != nil {
			return tmp.Error
		}
		return nil
	})
}

func (m *persistenceManagerImpl) RefreshVideoSourceStats(
	ctxt context.Context, id string, reqRespTargetID string, sourceLocalTime time.Time,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.Where(&videoSource{VideoSource: common.VideoSource{
			ID: id,
		}}).Updates(&videoSource{VideoSource: common.VideoSource{
			ReqRespTargetID: &reqRespTargetID,
			SourceLocalTime: sourceLocalTime,
		}}); tmp.Error != nil {
			return tmp.Error
		}
		return nil
	})
}

func (m *persistenceManagerImpl) DeleteVideoSource(ctxt context.Context, id string) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)
		if tmp := tx.Where(
			&liveStreamVideoSegment{VideoSegment: common.VideoSegment{SourceID: id}},
		).Delete(&liveStreamVideoSegment{}); tmp.Error != nil {
			return tmp.Error
		}
		if tmp := tx.Delete(&videoSource{VideoSource: common.VideoSource{ID: id}}); tmp.Error != nil {
			return tmp.Error
		}
		log.WithFields(logTags).WithField("id", id).Info("Delete video source")
		return nil
	})
}

// =====================================================================================
// Live Stream Video segments

func (m *persistenceManagerImpl) RegisterLiveStreamSegment(
	ctxt context.Context, sourceID string, segment hls.Segment,
) (string, error) {
	var newID string
	return newID, m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

		newID = ulid.Make().String()

		newEntry := liveStreamVideoSegment{
			VideoSegment: common.VideoSegment{
				ID:       newID,
				Segment:  segment,
				SourceID: sourceID,
			},
		}

		// Verify data
		if err := m.validator.Struct(&newEntry); err != nil {
			return err
		}

		// Insert entry
		if tmp := tx.Create(&newEntry); tmp.Error != nil {
			return tmp.Error
		}

		log.
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("id", newID).
			WithField("segment", segment.String()).
			Debug("Registered new video segment")
		return nil
	})
}

func (m *persistenceManagerImpl) BulkRegisterLiveStreamSegments(
	ctxt context.Context, sourceID string, segments []hls.Segment,
) (map[string]string, error) {
	newIDs := map[string]string{}
	return newIDs, m.db.Transaction(func(tx *gorm.DB) error {
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
				return err
			}
			newEntries = append(newEntries, newEntry)
			newIDs[segment.Name] = newID
		}

		// Insert entries
		if tmp := tx.Create(&newEntries); tmp.Error != nil {
			return tmp.Error
		}

		return nil
	})
}

func (m *persistenceManagerImpl) ListAllLiveStreamSegments(ctxt context.Context, sourceID string) (
	[]common.VideoSegment, error,
) {
	var results []common.VideoSegment
	return results, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []liveStreamVideoSegment
		if tmp := tx.Where(
			&liveStreamVideoSegment{VideoSegment: common.VideoSegment{SourceID: sourceID}},
		).Order("end_ts").Find(&entries); tmp.Error != nil {
			return tmp.Error
		}
		for _, entry := range entries {
			results = append(results, entry.VideoSegment)
		}
		return nil
	})
}

func (m *persistenceManagerImpl) GetLiveStreamSegment(
	ctxt context.Context, id string,
) (common.VideoSegment, error) {
	var result common.VideoSegment
	return result, m.db.Transaction(func(tx *gorm.DB) error {
		var entry liveStreamVideoSegment
		if tmp := tx.Where(
			&liveStreamVideoSegment{VideoSegment: common.VideoSegment{ID: id}},
		).First(&entry); tmp.Error != nil {
			return tmp.Error
		}
		result = entry.VideoSegment
		return nil
	})
}

func (m *persistenceManagerImpl) GetLiveStreamSegmentByName(
	ctxt context.Context, name string,
) (common.VideoSegment, error) {
	var result common.VideoSegment
	return result, m.db.Transaction(func(tx *gorm.DB) error {
		var entry liveStreamVideoSegment
		if tmp := tx.Where(
			&liveStreamVideoSegment{VideoSegment: common.VideoSegment{Segment: hls.Segment{Name: name}}},
		).First(&entry); tmp.Error != nil {
			return tmp.Error
		}
		result = entry.VideoSegment
		return nil
	})
}

func (m *persistenceManagerImpl) ListAllLiveStreamSegmentsAfterTime(
	ctxt context.Context, sourceID string, timestamp time.Time,
) ([]common.VideoSegment, error) {
	var results []common.VideoSegment
	return results, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []liveStreamVideoSegment
		if tmp := tx.
			Where(&liveStreamVideoSegment{VideoSegment: common.VideoSegment{SourceID: sourceID}}).
			Where("end_ts >= ?", timestamp).
			Order("end_ts").
			Find(&entries); tmp.Error != nil {
			return tmp.Error
		}
		for _, entry := range entries {
			results = append(results, entry.VideoSegment)
		}
		return nil
	})
}

func (m *persistenceManagerImpl) GetLatestLiveStreamSegments(
	ctxt context.Context, sourceID string, count int,
) ([]common.VideoSegment, error) {
	var results []common.VideoSegment
	return results, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []liveStreamVideoSegment
		if tmp := tx.
			Where(&liveStreamVideoSegment{VideoSegment: common.VideoSegment{SourceID: sourceID}}).
			Order("end_ts desc").
			Limit(count).
			Find(&entries); tmp.Error != nil {
			return tmp.Error
		}
		results = make([]common.VideoSegment, len(entries))
		for idx, entry := range entries {
			results[len(entries)-idx-1] = entry.VideoSegment
		}
		return nil
	})
}

func (m *persistenceManagerImpl) DeleteLiveStreamSegment(ctxt context.Context, id string) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.Where(
			&liveStreamVideoSegment{VideoSegment: common.VideoSegment{ID: id}},
		).Delete(&liveStreamVideoSegment{}); tmp.Error != nil {
			return tmp.Error
		}
		return nil
	})
}

func (m *persistenceManagerImpl) BulkDeleteLiveStreamSegment(ctxt context.Context, ids []string) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.Where("id in ?", ids).Delete(&liveStreamVideoSegment{}); tmp.Error != nil {
			return tmp.Error
		}
		return nil
	})
}

func (m *persistenceManagerImpl) PurgeOldLiveStreamSegments(
	ctxt context.Context, timeLimit time.Time,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.
			Where("end_ts < ?", timeLimit).
			Delete(&liveStreamVideoSegment{}); tmp.Error != nil {
			return tmp.Error
		}
		return nil
	})
}

// =====================================================================================
// Video Recording Sessions

func (m *persistenceManagerImpl) DefineRecordingSession(
	ctxt context.Context, sourceID string, alias, description *string, startTime time.Time,
) (string, error) {
	var newEntryID string
	return newEntryID, m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

		// Prepare new entry
		newEntryID = ulid.Make().String()

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
			return err
		}

		// Insert entry
		if tmp := tx.Create(&newEntry); tmp.Error != nil {
			return tmp.Error
		}

		log.
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("id", newEntryID).
			Info("Registered new video recording session")
		return nil
	})
}

func (m *persistenceManagerImpl) RecordKnownRecordingSession(
	ctxt context.Context, entry common.Recording,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
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
			return err
		}

		// Insert entry
		if tmp := tx.Create(&newEntry); tmp.Error != nil {
			return tmp.Error
		}

		log.
			WithFields(logTags).
			WithField("source-id", entry.SourceID).
			WithField("id", entry.ID).
			Info("Registered pre-existing video recording session")

		return nil
	})
}

func (m *persistenceManagerImpl) GetRecordingSession(
	ctxt context.Context, id string,
) (common.Recording, error) {
	var result common.Recording
	return result, m.db.Transaction(func(tx *gorm.DB) error {
		var entry recordingSession

		tmp := tx.Where(&recordingSession{Recording: common.Recording{ID: id}}).First(&entry)
		if tmp.Error != nil {
			return tmp.Error
		}

		result = entry.Recording
		return nil
	})
}

func (m *persistenceManagerImpl) ListRecordingSessions(
	ctxt context.Context,
) ([]common.Recording, error) {
	var results []common.Recording
	return results, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []recordingSession

		if tmp := tx.Find(&entries); tmp.Error != nil {
			return tmp.Error
		}

		for _, entry := range entries {
			results = append(results, entry.Recording)
		}
		return nil
	})
}

func (m *persistenceManagerImpl) ListRecordingSessionsOfSource(
	ctxt context.Context, sourceID string, active bool,
) ([]common.Recording, error) {
	var results []common.Recording
	return results, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []recordingSession

		query := tx.Where(&recordingSession{Recording: common.Recording{SourceID: sourceID}})
		if active {
			query = query.Where(&recordingSession{Recording: common.Recording{Active: 1}})
		}
		if tmp := query.Find(&entries); tmp.Error != nil {
			return tmp.Error
		}

		for _, entry := range entries {
			results = append(results, entry.Recording)
		}
		return nil
	})
}

func (m *persistenceManagerImpl) MarkEndOfRecordingSession(
	ctxt context.Context, id string, endTime time.Time,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

		if tmp := tx.Where(
			&recordingSession{Recording: common.Recording{ID: id}},
		).Updates(
			&recordingSession{Recording: common.Recording{Active: -1, EndTime: endTime}},
		); tmp.Error != nil {
			return tmp.Error
		}

		log.WithFields(logTags).WithField("recording", id).Info("Marking recording as complete")

		return nil
	})
}

func (m *persistenceManagerImpl) UpdateRecordingSession(
	ctxt context.Context, newSetting common.Recording,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.Where(
			&recordingSession{Recording: common.Recording{ID: newSetting.ID}},
		).Updates(
			&recordingSession{Recording: common.Recording{
				Alias: newSetting.Alias, Description: newSetting.Description,
			}},
		); tmp.Error != nil {
			return tmp.Error
		}
		return nil
	})
}

func (m *persistenceManagerImpl) DeleteRecordingSession(ctxt context.Context, id string) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

		// Get the recording entry first
		var recording recordingSession
		tmp := tx.Where(
			&recordingSession{Recording: common.Recording{ID: id}},
		).Preload("Segments").First(&recording)
		if tmp.Error != nil {
			return tmp.Error
		}

		// Delete the segments associated along with the recording
		if len(recording.Segments) > 0 {
			if tmp = tx.Select(recording.Segments).Delete(&recording); tmp.Error != nil {
				return tmp.Error
			}
		} else if tmp = tx.Delete(&recording); tmp.Error != nil {
			return tmp.Error
		}

		log.WithFields(logTags).WithField("recording", id).Info("Deleted recording session")

		return nil
	})
}
