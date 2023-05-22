package db

import (
	"context"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/apex/log"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// Manager database access layer
type Manager interface {
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

}

// managerImpl implements Manager
type managerImpl struct {
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
func NewManager(dbDialector gorm.Dialector, logLevel logger.LogLevel) (Manager, error) {
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
	return &managerImpl{
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

func (m *managerImpl) Ready(ctxt context.Context) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		tmp := tx.Find(&[]videoSource{}).Limit(1)
		return tmp.Error
	})
}

// =====================================================================================
// Video sources

func (m *managerImpl) DefineVideoSource(
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

func (m *managerImpl) RecordKnownVideoSource(
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

func (m *managerImpl) GetVideoSource(ctxt context.Context, id string) (common.VideoSource, error) {
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

func (m *managerImpl) ListVideoSources(ctxt context.Context) ([]common.VideoSource, error) {
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

func (m *managerImpl) UpdateVideoSource(ctxt context.Context, newSetting common.VideoSource) error {
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

func (m *managerImpl) DeleteVideoSource(ctxt context.Context, id string) error {
	return m.db.Transaction(func(tx *gorm.DB) error {
		logTags := m.GetLogTagsForContext(ctxt)
		if tmp := tx.Delete(&videoSource{VideoSource: common.VideoSource{ID: id}}); tmp.Error != nil {
			return nil
		}
		log.WithFields(logTags).WithField("id", id).Info("Delete video source")
		return nil
	})
}
