package edge

import (
	"context"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/db"
	"github.com/alwitt/livemix/utils"
	"github.com/apex/log"
)

// VideoSourceOperator video source operations manager
type VideoSourceOperator interface {
	/*
		Ready check whether the DB connection is working

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
		ChangeVideoSourceStreamState change the streaming state for a video source

			@param ctxt context.Context - execution context
			@param id string - source ID
			@param streaming int - new streaming state
	*/
	ChangeVideoSourceStreamState(ctxt context.Context, id string, streaming int) error
}

// videoSourceOperatorImpl implements VideoSourceOperator
type videoSourceOperatorImpl struct {
	goutils.Component
	self                common.VideoSource
	selfReqRespTargetID string
	dbConns             db.ConnectionManager
	broadcastClient     utils.Broadcaster
	reportTriggerTimer  goutils.IntervalTimer
	wg                  sync.WaitGroup
	workerCtxt          context.Context
	workerCtxtCancel    context.CancelFunc
}

/*
NewManager define a new video source operator

	@param parentCtxt context.Context - parent execution context
	@param self common.VideoSource - information report the video source being operated
	@param rrTargetID string - request-response target this edge node will respond on
	@param dbConns db.ConnectionManager - DB connection manager
	@param broadcastClient utils.Broadcaster - message broadcast client
	@param reportInterval time.Duration - status report interval
	@return new operator
*/
func NewManager(
	parentCtxt context.Context,
	self common.VideoSource,
	rrTargetID string,
	dbConns db.ConnectionManager,
	broadcastClient utils.Broadcaster,
	reportInterval time.Duration,
) (VideoSourceOperator, error) {
	logTags := log.Fields{
		"module": "edge", "component": "video-source-operator", "instance": self.Name,
	}

	instance := &videoSourceOperatorImpl{
		Component: goutils.Component{
			LogTags: logTags,
			LogTagModifiers: []goutils.LogMetadataModifier{
				goutils.ModifyLogMetadataByRestRequestParam,
			},
		},
		self:                self,
		selfReqRespTargetID: rrTargetID,
		dbConns:             dbConns,
		broadcastClient:     broadcastClient,
		wg:                  sync.WaitGroup{},
	}
	instance.workerCtxt, instance.workerCtxtCancel = context.WithCancel(parentCtxt)

	// Define periodic timer for sending video source status reports to system control node
	timerLogTags := log.Fields{"sub-module": "status-report-timer"}
	for lKey, lVal := range logTags {
		timerLogTags[lKey] = lVal
	}
	reportTriggerTimer, err := goutils.GetIntervalTimerInstance(
		instance.workerCtxt, &instance.wg, timerLogTags,
	)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Unable to define status report trigger timer")
		return nil, err
	}
	instance.reportTriggerTimer = reportTriggerTimer

	// Make first status report
	if err := instance.sendSourceStatusReport(); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to send first status report")
		return nil, err
	}

	// Start timer to periodically send video source status reports to the system control node
	err = reportTriggerTimer.Start(reportInterval, instance.sendSourceStatusReport, false)
	if err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to start status report trigger timer")
		return nil, err
	}

	return instance, nil
}

func (o *videoSourceOperatorImpl) Ready(ctxt context.Context) error {
	dbClient := o.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.Ready(ctxt)
}

func (o *videoSourceOperatorImpl) Stop(ctxt context.Context) error {
	o.workerCtxtCancel()
	if err := o.reportTriggerTimer.Stop(); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &o.wg, time.Second*5)
}

func (o *videoSourceOperatorImpl) RecordKnownVideoSource(
	ctxt context.Context,
	id, name string,
	segmentLen int,
	playlistURI, description *string,
	streaming int,
) error {
	dbClient := o.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.RecordKnownVideoSource(
		ctxt, id, name, segmentLen, playlistURI, description, streaming,
	)
}

func (o *videoSourceOperatorImpl) ChangeVideoSourceStreamState(
	ctxt context.Context, id string, streaming int,
) error {
	dbClient := o.dbConns.NewPersistanceManager()
	defer dbClient.Close()
	return dbClient.ChangeVideoSourceStreamState(ctxt, id, streaming)
}

func (o *videoSourceOperatorImpl) sendSourceStatusReport() error {
	logTags := o.GetLogTagsForContext(o.workerCtxt)

	report := ipc.NewVideoSourceStatusReport(o.self.ID, o.selfReqRespTargetID, time.Now().UTC())

	if err := o.broadcastClient.Broadcast(o.workerCtxt, &report); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to send status report message")
		return err
	}

	return nil
}
