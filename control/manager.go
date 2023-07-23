package control

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
)

// SystemManager system operations manager
type SystemManager interface {
	/*
		Ready check whether the manager is ready

			@param ctxt context.Context - execution context
	*/
	Ready(ctxt context.Context) error

	/*
		Stop stop any support background tasks which were started

			@param ctxt context.Context - execution context
	*/
	Stop(ctxt context.Context) error

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
			@param force bool - force through the change regardless whether the video source
			    is accepting inbound requests.
	*/
	MarkEndOfRecordingSession(ctxt context.Context, id string, endTime time.Time, force bool) error

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
			@param force bool - force through the change regardless whether the video source
			    is accepting inbound requests.
	*/
	DeleteRecordingSession(ctxt context.Context, id string, force bool) error

	/*
		StopAllActiveRecordingOfSource stop any active recording sessions associated with a source

			@param ctxt context.Context - execution context
			@param id string - source entry ID
			@param currentTime time.Time - current timestamp
	*/
	StopAllActiveRecordingOfSource(ctxt context.Context, id string, currentTime time.Time) error

	// =====================================================================================
	// Video Recording Segments

	/*
		ListAllSegmentsOfRecording fetch all video segments belonging to one recording session

			@param ctxt context.Context - execution context
			@param recordingID string - video recording session ID
			@returns set of video segments
	*/
	ListAllSegmentsOfRecording(
		ctxt context.Context, recordingID string,
	) ([]common.VideoSegment, error)

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

	/*
		PurgeUnassociatedRecordingSegments trigger to purge recording segments unassociated
		with any recordings from storage
	*/
	PurgeUnassociatedRecordingSegments() error
}

// systemManagerImpl implements SystemManager
type systemManagerImpl struct {
	goutils.Component
	dbConns                     db.ConnectionManager
	rrClient                    EdgeRequestClient
	s3                          utils.S3Client
	maxAgeForSourceStatusReport time.Duration
	segmentCleanupTimer         goutils.IntervalTimer
	wg                          sync.WaitGroup
	workerCtxt                  context.Context
	workerCtxtCancel            context.CancelFunc
}

/*
NewManager define a new system manager

	@param parentCtxt context.Context - parent execution context
	@param dbConns db.ConnectionManager - DB connection manager
	@param rrClient EdgeRequestClient - request-response client
	@param s3 utils.S3Client - S3 operation client
	@param maxAgeForSourceStatusReport time.Duration - for the system to send a requests to a
	    particular video source, this source must have sent out a video source status report
	    within this time window before a request is made. If not, the video source is treated as
	    connected.
	@param segmentCleanupInt time.Duration - time interval between segment cleanup runs
	@returns new manager
*/
func NewManager(
	parentCtxt context.Context,
	dbConns db.ConnectionManager,
	rrClient EdgeRequestClient,
	s3 utils.S3Client,
	maxAgeForSourceStatusReport time.Duration,
	segmentCleanupInt time.Duration,
) (SystemManager, error) {
	logTags := log.Fields{"module": "control", "component": "system-manager"}

	instance := &systemManagerImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		dbConns:                     dbConns,
		rrClient:                    rrClient,
		s3:                          s3,
		maxAgeForSourceStatusReport: maxAgeForSourceStatusReport,
	}
	instance.workerCtxt, instance.workerCtxtCancel = context.WithCancel(parentCtxt)

	// Define periodic timer for clearing out recording segments not associated with any
	// recordings
	timerLogTags := log.Fields{"sub-module": "recording-segment-cleanup-timer"}
	for lKey, lVal := range logTags {
		timerLogTags[lKey] = lVal
	}
	cleanupTimer, err := goutils.GetIntervalTimerInstance(
		instance.workerCtxt, &instance.wg, timerLogTags,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to define segment cleanup timer")
		return nil, err
	}
	instance.segmentCleanupTimer = cleanupTimer

	// -----------------------------------------------------------------------------
	// Start timer to periodically cleanup recording segments not related to any recordings
	err = cleanupTimer.Start(segmentCleanupInt, instance.PurgeUnassociatedRecordingSegments, false)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to start segment cleanup timer")
		return nil, err
	}

	return instance, nil
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

func (m *systemManagerImpl) Stop(ctxt context.Context) error {
	m.workerCtxtCancel()
	if err := m.segmentCleanupTimer.Stop(); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &m.wg, time.Second*5)
}

// =====================================================================================
// Video sources

func (m *systemManagerImpl) Ready(ctxt context.Context) error {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.Ready(ctxt)
}

func (m *systemManagerImpl) DefineVideoSource(
	ctxt context.Context, name string, segmentLen int, playlistURI, description *string,
) (string, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.DefineVideoSource(ctxt, name, segmentLen, playlistURI, description)
}

func (m *systemManagerImpl) GetVideoSource(
	ctxt context.Context, id string,
) (common.VideoSource, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.GetVideoSource(ctxt, id)
}

func (m *systemManagerImpl) GetVideoSourceByName(
	ctxt context.Context, name string,
) (common.VideoSource, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.GetVideoSourceByName(ctxt, name)
}

func (m *systemManagerImpl) ListVideoSources(ctxt context.Context) ([]common.VideoSource, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.ListVideoSources(ctxt)
}

func (m *systemManagerImpl) UpdateVideoSource(
	ctxt context.Context, newSetting common.VideoSource,
) error {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.UpdateVideoSource(ctxt, newSetting)
}

func (m *systemManagerImpl) ChangeVideoSourceStreamState(
	ctxt context.Context, id string, streaming int,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Fetch entry
	entry, err := dbClient.GetVideoSource(ctxt, id)
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

	// Update persistence
	if err := dbClient.ChangeVideoSourceStreamState(ctxt, id, streaming); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", id).
			Error("Failed to persist streaming state change")
		return err
	}

	// Request state change
	if err := m.rrClient.ChangeVideoStreamingState(ctxt, entry, streaming); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", id).
			Error("Streaming state change request failed")
		dbClient.MarkExternalError(err)
		return err
	}

	return nil
}

func (m *systemManagerImpl) DeleteVideoSource(ctxt context.Context, id string) error {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.DeleteVideoSource(ctxt, id)
}

// =====================================================================================
// Video Recording Sessions

func (m *systemManagerImpl) DefineRecordingSession(
	ctxt context.Context, sourceID string, alias, description *string, startTime time.Time,
) (string, error) {
	logTags := m.GetLogTagsForContext(ctxt)

	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	source, err := dbClient.GetVideoSource(ctxt, sourceID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			Error("Unable to find video source")
		return "", err
	}

	// Verify that it is possible to make the request
	if err := m.canRequestVideoSource(source); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			Error("Can't make request to source")
		return "", err
	}

	// Define new recording entry
	recordingID, err := dbClient.DefineRecordingSession(ctxt, sourceID, alias, description, startTime)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			Error("Unable to define new recording for source")
		return "", err
	}
	recording, err := dbClient.GetRecordingSession(ctxt, recordingID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("recording", recordingID).
			Error("Unable to retrieve newly defined recording entry")
		return "", err
	}

	log.
		WithFields(logTags).
		WithField("source-id", sourceID).
		WithField("recording", recordingID).
		Info("Defined new recording entry")

	// Request the source to start this recording
	if err := m.rrClient.StartRecordingSession(ctxt, source, recording); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", sourceID).
			WithField("recording", recordingID).
			Error("Unable to command source to start recording")
		dbClient.MarkExternalError(err)
		return "", err
	}

	log.
		WithFields(logTags).
		WithField("source-id", sourceID).
		WithField("recording", recordingID).
		Info("Commanded video source to start new recording")

	return recordingID, nil
}

func (m *systemManagerImpl) GetRecordingSession(
	ctxt context.Context, id string,
) (common.Recording, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.GetRecordingSession(ctxt, id)
}

func (m *systemManagerImpl) GetRecordingSessionByAlias(
	ctxt context.Context, alias string,
) (common.Recording, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.GetRecordingSessionByAlias(ctxt, alias)
}

func (m *systemManagerImpl) ListRecordingSessions(ctxt context.Context) ([]common.Recording, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.ListRecordingSessions(ctxt)
}

func (m *systemManagerImpl) ListRecordingSessionsOfSource(
	ctxt context.Context, sourceID string, active bool,
) ([]common.Recording, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.ListRecordingSessionsOfSource(ctxt, sourceID, active)
}

func (m *systemManagerImpl) MarkEndOfRecordingSession(
	ctxt context.Context, id string, endTime time.Time, force bool,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Get the recording entry
	recording, err := dbClient.GetRecordingSession(ctxt, id)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("recording", id).
			Error("Unable to retrieve recording entry")
		return err
	}

	// Get the source supporting the recording
	source, err := dbClient.GetVideoSource(ctxt, recording.SourceID)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", recording.SourceID).
			WithField("recording", id).
			Error("Unable to retrieve video source entry")
		return err
	}

	// Stop the recording entry
	if err := dbClient.MarkEndOfRecordingSession(ctxt, id, endTime); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", source.ID).
			WithField("recording", id).
			Error("Failed to mark recording as ended")
		return err
	}

	// Verify that it is possible to make the request
	if err := m.canRequestVideoSource(source); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", source.ID).
			Error("Can't make request to source")
		if force {
			// Ignore this error
			return nil
		}
		dbClient.MarkExternalError(err)
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", source.ID).
		WithField("recording", id).
		Info("Requesting source to stop recording")

	// Request the source to stop this recording
	if err := m.rrClient.StopRecordingSession(ctxt, source, recording.ID, endTime); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", source.ID).
			WithField("recording", id).
			Error("Unable to command source to stop recording")
		if force {
			// Ignore this error
			return nil
		}
		dbClient.MarkExternalError(err)
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", source.ID).
		WithField("recording", id).
		Info("Source has stop recording")

	return nil
}

func (m *systemManagerImpl) UpdateRecordingSession(
	ctxt context.Context, newSetting common.Recording,
) error {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.UpdateRecordingSession(ctxt, newSetting)
}

func (m *systemManagerImpl) DeleteRecordingSession(
	ctxt context.Context, id string, force bool,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	currentTime := time.Now().UTC()
	if err := m.MarkEndOfRecordingSession(ctxt, id, currentTime, force); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("recording", id).
			Error("Unable to mark recording session ended")
		return err
	}

	// Delete the entry
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.DeleteRecordingSession(ctxt, id)
}

func (m *systemManagerImpl) StopAllActiveRecordingOfSource(
	ctxt context.Context, id string, currentTime time.Time,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	source, err := dbClient.GetVideoSource(ctxt, id)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", id).
			Error("Unable to find video source")
		return err
	}

	// Verify that it is possible to make the request
	if err := m.canRequestVideoSource(source); err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", id).
			Error("Can't make request to source")
		return err
	}

	log.
		WithFields(logTags).
		WithField("source-id", id).
		Info("Stopping all associated recording sessions")

	// Get the active recording session for a source
	sessions, err := dbClient.ListRecordingSessionsOfSource(ctxt, id, true)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			WithField("source-id", id).
			Error("Unable to list active recording sessions of video source")
		return err
	}

	if len(sessions) == 0 {
		return nil
	}

	// Mark end of recording in persistence
	for _, session := range sessions {
		if err := dbClient.MarkEndOfRecordingSession(ctxt, session.ID, currentTime); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source-id", id).
				WithField("recording-session", session.ID).
				Error("Failed to mark recording session as ended")
		}
	}

	// Request edge nodes to stop recording
	for _, session := range sessions {
		if err := m.rrClient.StopRecordingSession(ctxt, source, session.ID, currentTime); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("source-id", id).
				WithField("recording-session", session.ID).
				Error("Stop recording session request failed")
		}
	}

	log.
		WithFields(logTags).
		WithField("source-id", id).
		Info("Stopped all associated recording sessions")

	return nil
}

func (m *systemManagerImpl) ListAllSegmentsOfRecording(
	ctxt context.Context, recordingID string,
) ([]common.VideoSegment, error) {
	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.ListAllSegmentsOfRecording(ctxt, recordingID)
}

// =====================================================================================
// Utilities

func (m *systemManagerImpl) ProcessBroadcastMsgs(
	ctxt context.Context, pubTimestamp time.Time, msg []byte, metadata map[string]string,
) error {
	logTags := m.GetLogTagsForContext(ctxt)

	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Parse the message
	parsed, err := ipc.ParseRawMessage(msg)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to parse the broadcast message")
		dbClient.MarkExternalError(err)
		return err
	}

	// Process the message based on type
	msgType := reflect.TypeOf(parsed)
	switch reflect.TypeOf(parsed) {
	case reflect.TypeOf(ipc.VideoSourceStatusReport{}):
		statusReport := parsed.(ipc.VideoSourceStatusReport)
		// Record
		if err := dbClient.RefreshVideoSourceStats(
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

	case reflect.TypeOf(ipc.RecordingSegmentReport{}):
		newSegmentReport := parsed.(ipc.RecordingSegmentReport)
		// Record the new segments
		if err := dbClient.RegisterRecordingSegments(
			ctxt, newSegmentReport.RecordingIDs, newSegmentReport.Segments,
		); err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("recordings", newSegmentReport.RecordingIDs).
				Error("Unable to record new recording video segments report")
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

func (m *systemManagerImpl) PurgeUnassociatedRecordingSegments() error {
	logTags := m.GetLogTagsForContext(m.workerCtxt)

	dbClient := m.dbConns.NewPersistanceManager()
	defer dbClient.Close()

	// Purge the un-associated segments
	segmentsToDelete, err := dbClient.PurgeUnassociatedRecordingSegments(m.workerCtxt)
	if err != nil {
		log.
			WithError(err).
			WithFields(logTags).
			Error("Failed to purge recording segments not related to any recordings")
		return err
	}

	if len(segmentsToDelete) == 0 {
		return nil
	}

	log.
		WithFields(logTags).
		WithField("segments", len(segmentsToDelete)).
		Info("Found recording segments un-associated with any recording")

	// Remove the segments from object storage
	segmentByBucket := map[string][]string{}
	for _, oneSegment := range segmentsToDelete {
		url, err := url.Parse(oneSegment.URI)
		if err != nil {
			log.
				WithError(err).
				WithFields(logTags).
				WithField("segment-uri", oneSegment.URI).
				Error("Unable to parse segment URI")
		}
		segmentBucket := url.Host
		segmentObjectKey := utils.CleanupObjectKey(url.Path)

		// Record the segment for deletion
		if _, ok := segmentByBucket[segmentBucket]; !ok {
			segmentByBucket[segmentBucket] = []string{}
		}
		segmentByBucket[segmentBucket] = append(segmentByBucket[segmentBucket], segmentObjectKey)
	}

	for bucketName, segments := range segmentByBucket {
		log.
			WithFields(logTags).
			WithField("recording-bucket", bucketName).
			WithField("segments", len(segments)).
			Info("Deleting unassociated recording segments in bucket")

		// Purge from S3
		errs := m.s3.DeleteObjects(m.workerCtxt, bucketName, segments)
		if len(errs) > 0 {
			// Errors have occurred during deletion
			logHandle := log.WithFields(logTags)
			for _, oneErr := range errs {
				logHandle = logHandle.WithError(oneErr)
				dbClient.MarkExternalError(oneErr)
			}
			logHandle.Error("Failed to purge unassociated recording segments")
			return errs[0]
		}

		log.
			WithFields(logTags).
			WithField("recording-bucket", bucketName).
			WithField("segments", len(segments)).
			Info("Deleted unassociated recording segments in bucket")
	}

	return nil
}
