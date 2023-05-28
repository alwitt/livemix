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
			@param playlistURI string - video source playlist URI
			@param description *string - optionally, source description
			@returns new source entry ID
	*/
	DefineVideoSource(
		ctxt context.Context, name, playlistURI string, description *string,
	) (string, error)

	/*
		RecordKnownVideoSource create record for a known video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
			@param name string - source name
			@param playlistURI string - video source playlist URI
			@param description *string - optionally, source description
	*/
	RecordKnownVideoSource(
		ctxt context.Context, id, name, playlistURI string, description *string,
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
		DeleteVideoSource delete a video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
	*/
	DeleteVideoSource(ctxt context.Context, id string) error

	// =====================================================================================
	// Video segments

	/*
		RegisterSegment record a new segment with a source

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param segment hls.Segment - video segment parameters
			@returns new segment entry ID
	*/
	RegisterSegment(ctxt context.Context, sourceID string, segment hls.Segment) (string, error)

	/*
		BulkRegisterSegments register multiple segment in bulk

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param segments []hls.Segment - set of video segments to insert
			@returns new segment entry IDs
	*/
	BulkRegisterSegments(
		ctxt context.Context, sourceID string, segments []hls.Segment,
	) (map[string]string, error)

	/*
		ListAllSegments fetch all video segments

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@returns list of segments
	*/
	ListAllSegments(ctxt context.Context, sourceID string) ([]common.VideoSegment, error)

	/*
		GetVideoSegment get a video segment

			@param ctxt context.Context - execution context
			@param id string - video segment ID
			@returns segment
	*/
	GetSegment(ctxt context.Context, id string) (common.VideoSegment, error)

	/*
		ListAllSegmentsAfterTime fetch all video segments which have a stop timestamp
		after a timestamp

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param timestamp time.Time - timestamp to check against
			@returns list of segments
	*/
	ListAllSegmentsAfterTime(
		ctxt context.Context, sourceID string, timestamp time.Time,
	) ([]common.VideoSegment, error)

	/*
		ListAllSegmentsBeforeTime fetch all video segments which have a stop timestamp
		before a timestamp

			@param ctxt context.Context - execution context
			@param sourceID string - video source ID
			@param timestamp time.Time - timestamp to check against
			@returns list of segments
	*/
	ListAllSegmentsBeforeTime(
		ctxt context.Context, sourceID string, timestamp time.Time,
	) ([]common.VideoSegment, error)

	/*
		DeleteSegment delete a segment

			@param ctxt context.Context - execution context
			@param id string - video segment ID
	*/
	DeleteSegment(ctxt context.Context, id string) error

	/*
		BulkDeleteSegment delete a group of segments

			@param ctxt context.Context - execution context
			@param ids []string - video segment IDs
	*/
	BulkDeleteSegment(ctxt context.Context, ids []string) error
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
	if err := db.AutoMigrate(&videoSegment{}); err != nil {
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
	ctxt context.Context, name, playlistURI string, description *string,
) (string, error) {
	newEntryID := ""
	return newEntryID, m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

		// Prepare new entry
		newEntryID = uuid.NewString()
		newEntry := videoSource{
			VideoSource: common.VideoSource{
				ID:          newEntryID,
				Name:        name,
				Description: description,
				PlaylistURI: playlistURI,
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
	ctxt context.Context, id, name, playlistURI string, description *string,
) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

		// Prepare entry
		entry := videoSource{
			VideoSource: common.VideoSource{
				ID:          id,
				Name:        name,
				Description: description,
				PlaylistURI: playlistURI,
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

func (m *persistenceManagerImpl) GetVideoSource(ctxt context.Context, id string) (common.VideoSource, error) {
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

func (m *persistenceManagerImpl) GetVideoSourceByName(ctxt context.Context, name string) (common.VideoSource, error) {
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

func (m *persistenceManagerImpl) UpdateVideoSource(ctxt context.Context, newSetting common.VideoSource) error {
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

func (m *persistenceManagerImpl) DeleteVideoSource(ctxt context.Context, id string) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)
		if tmp := tx.Where("source = ?", id).Delete(&videoSegment{}); tmp.Error != nil {
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
// Video segments

func (m *persistenceManagerImpl) RegisterSegment(
	ctxt context.Context, sourceID string, segment hls.Segment,
) (string, error) {
	var newID string
	return newID, m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)

		newID = ulid.Make().String()

		newEntry := videoSegment{
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

func (m *persistenceManagerImpl) BulkRegisterSegments(
	ctxt context.Context, sourceID string, segments []hls.Segment,
) (map[string]string, error) {
	newIDs := map[string]string{}
	return newIDs, m.db.Transaction(func(tx *gorm.DB) error {
		newEntries := []videoSegment{}
		// Prepare entries for batch insert
		for _, segment := range segments {
			newID := ulid.Make().String()
			newEntry := videoSegment{
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

func (m *persistenceManagerImpl) ListAllSegments(ctxt context.Context, sourceID string) (
	[]common.VideoSegment, error,
) {
	var results []common.VideoSegment
	return results, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []videoSegment
		if tmp := tx.Where("source = ?", sourceID).Order("id").Find(&entries); tmp.Error != nil {
			return tmp.Error
		}
		for _, entry := range entries {
			results = append(results, entry.VideoSegment)
		}
		return nil
	})
}

func (m *persistenceManagerImpl) GetSegment(ctxt context.Context, id string) (common.VideoSegment, error) {
	var result common.VideoSegment
	return result, m.db.Transaction(func(tx *gorm.DB) error {
		var entry videoSegment
		if tmp := tx.Where("id = ?", id).First(&entry); tmp.Error != nil {
			return tmp.Error
		}
		result = entry.VideoSegment
		return nil
	})
}

func (m *persistenceManagerImpl) ListAllSegmentsAfterTime(
	ctxt context.Context, sourceID string, timestamp time.Time,
) ([]common.VideoSegment, error) {
	var results []common.VideoSegment
	return results, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []videoSegment
		if tmp := tx.
			Where("source = ?", sourceID).
			Where("stop >= ?", timestamp).
			Order("id").
			Find(&entries); tmp.Error != nil {
			return tmp.Error
		}
		for _, entry := range entries {
			results = append(results, entry.VideoSegment)
		}
		return nil
	})
}

func (m *persistenceManagerImpl) ListAllSegmentsBeforeTime(
	ctxt context.Context, sourceID string, timestamp time.Time,
) ([]common.VideoSegment, error) {
	var results []common.VideoSegment
	return results, m.db.Transaction(func(tx *gorm.DB) error {
		var entries []videoSegment
		if tmp := tx.
			Where("source = ?", sourceID).
			Where("stop <= ?", timestamp).
			Order("id").
			Find(&entries); tmp.Error != nil {
			return tmp.Error
		}
		for _, entry := range entries {
			results = append(results, entry.VideoSegment)
		}
		return nil
	})
}

func (m *persistenceManagerImpl) DeleteSegment(ctxt context.Context, id string) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.Where("id = ?", id).Delete(&videoSegment{}); tmp.Error != nil {
			return tmp.Error
		}
		return nil
	})
}

func (m *persistenceManagerImpl) BulkDeleteSegment(ctxt context.Context, ids []string) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		if tmp := tx.Where("id in ?", ids).Delete(&videoSegment{}); tmp.Error != nil {
			return tmp.Error
		}
		return nil
	})
}
