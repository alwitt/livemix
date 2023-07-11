package control

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/db"
	"github.com/apex/log"
)

// SystemManager system operations manager
type SystemManager interface {
	/*
		Ready check whether the manager is ready

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
		DeleteVideoSource delete a video source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
	*/
	DeleteVideoSource(ctxt context.Context, id string) error

	// =====================================================================================
	// Utilities

	/*
		ProcessBroadcastMsgs process received broadcast messages

			@param ctxt context.Context - execution context
			@param pubTimestamp time.Time - timestamp when the message was published
			@param msg []byte - broadcast message payload
			@param metadata map[string]string - broadcast message metadata
	*/
	ProcessBroadcastMsgs(
		ctxt context.Context, pubTimestamp time.Time, msg []byte, metadata map[string]string,
	) error
}

// systemManagerImpl implements SystemManager
type systemManagerImpl struct {
	goutils.Component
	db                          db.PersistenceManager
	rrClient                    EdgeRequestClient
	maxAgeForSourceStatusReport time.Duration
}

/*
NewManager define a new system manager

	@param dbClient db.PersistenceManager - persistence manager
	@param rrClient EdgeRequestClient - request-response client
	@param maxAgeForSourceStatusReport time.Duration - for the system to send a requests to a
	    particular video source, this source must have sent out a video source status report
	    within this time window before a request is made. If not, the video source is treated as
	    connected.
	@returns new manager
*/
func NewManager(
	dbClient db.PersistenceManager,
	rrClient EdgeRequestClient,
	maxAgeForSourceStatusReport time.Duration,
) (SystemManager, error) {
	logTags := log.Fields{"module": "control", "component": "system-manager"}
	return &systemManagerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		db:                          dbClient,
		rrClient:                    rrClient,
		maxAgeForSourceStatusReport: maxAgeForSourceStatusReport,
	}, nil
}

func (m *systemManagerImpl) canRequestVideoSource(source common.VideoSource) error {
	currentTime := time.Now().UTC()

	if source.ReqRespTargetID == nil {
		return fmt.Errorf("video source '%s' have not reported a request-response target ID", source.ID)
	}

	if source.SourceLocalTime.Add(m.maxAgeForSourceStatusReport).Before(currentTime) {
		return fmt.Errorf(
			"video source '%s' have not sent a status report within last %s",
			source.ID,
			m.maxAgeForSourceStatusReport.String(),
		)
	}

	return nil
}

func (m *systemManagerImpl) Ready(ctxt context.Context) error {
	return m.db.Ready(ctxt)
}

func (m *systemManagerImpl) DefineVideoSource(
	ctxt context.Context, name string, segmentLen int, playlistURI, description *string,
) (string, error) {
	return m.db.DefineVideoSource(ctxt, name, segmentLen, playlistURI, description)
}

func (m *systemManagerImpl) GetVideoSource(
	ctxt context.Context, id string,
) (common.VideoSource, error) {
	return m.db.GetVideoSource(ctxt, id)
}

func (m *systemManagerImpl) GetVideoSourceByName(
	ctxt context.Context, name string,
) (common.VideoSource, error) {
	return m.db.GetVideoSourceByName(ctxt, name)
}

func (m *systemManagerImpl) ListVideoSources(ctxt context.Context) ([]common.VideoSource, error) {
	return m.db.ListVideoSources(ctxt)
}

func (m *systemManagerImpl) UpdateVideoSource(
	ctxt context.Context, newSetting common.VideoSource,
) error {
	return m.db.UpdateVideoSource(ctxt, newSetting)
}

func (m *systemManagerImpl) ChangeVideoSourceStreamState(
	ctxt context.Context, id string, streaming int,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	// Fetch entry
	entry, err := m.db.GetVideoSource(ctxt, id)
	if err != nil {
		log.WithError(err).WithFields(logTags).Errorf("Unable to find video source '%s'", id)
		return err
	}

	// Verify that it is possible to make the request
	if err := m.canRequestVideoSource(entry); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", id).
			Error("Can't make request to source")
		return err
	}

	// Request state change first
	if err := m.rrClient.ChangeVideoStreamingState(ctxt, entry, streaming); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", id).
			Error("Streaming state change request failed")
		return err
	}

	// Update persistence
	if err := m.db.ChangeVideoSourceStreamState(ctxt, id, streaming); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", id).
			Error("Failed to persist streaming state change")
		return err
	}

	return nil
}

func (m *systemManagerImpl) DeleteVideoSource(ctxt context.Context, id string) error {
	return m.db.DeleteVideoSource(ctxt, id)
}

func (m *systemManagerImpl) ProcessBroadcastMsgs(
	ctxt context.Context, pubTimestamp time.Time, msg []byte, metadata map[string]string,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	// Parse the message
	parsed, err := ipc.ParseRawMessage(msg)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to parse the broadcast message")
		return err
	}

	// Process the message based on type
	msgType := reflect.TypeOf(parsed)
	switch reflect.TypeOf(parsed) {
	case reflect.TypeOf(ipc.VideoSourceStatusReport{}):
		statusReport := parsed.(ipc.VideoSourceStatusReport)
		// Record
		if err := m.db.RefreshVideoSourceStats(
			ctxt,
			statusReport.SourceID,
			statusReport.RequestResponseTargetID,
			statusReport.LocalTimestamp,
		); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source-id", statusReport.SourceID).
				Error("Unable to record new video source status report")
			return err
		}

	default:
		log.
			WithFields(logTags).
			WithField("msg-type", msgType).
			Debug("Ignoring unsupported broadcast message type")
	}

	return nil
}
