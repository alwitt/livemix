package edge

import (
	"context"
	"sync"
	"time"

	"github.com/alwitt/goutils"
	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/db"
	"github.com/apex/log"
)

// VideoSourceStatusReportCB function signature of callback for forwarding video status reports
type VideoSourceStatusReportCB func(ctxt context.Context, report ipc.VideoSourceStatusReport) error

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
			@param playlistURI *string - video source playlist URI
			@param description *string - optionally, source description
			@param streaming int - whether the video source is currently streaming
	*/
	RecordKnownVideoSource(
		ctxt context.Context, id, name string, playlistURI, description *string, streaming int,
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
	db                  db.PersistenceManager
	reportStatus        VideoSourceStatusReportCB
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
	@param dbClient db.PersistenceManager - persistence manager
	@param reportCB VideoSourceStatusReportCB - video source status report forwarding callback
	@param reportInterval time.Duration - status report interval
	@return new operator
*/
func NewManager(
	parentCtxt context.Context,
	self common.VideoSource,
	rrTargetID string,
	dbClient db.PersistenceManager,
	reportCB VideoSourceStatusReportCB,
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
		db:                  dbClient,
		reportStatus:        reportCB,
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
	return o.db.Ready(ctxt)
}

func (o *videoSourceOperatorImpl) Stop(ctxt context.Context) error {
	o.workerCtxtCancel()
	if err := o.reportTriggerTimer.Stop(); err != nil {
		return err
	}
	return goutils.TimeBoundedWaitGroupWait(ctxt, &o.wg, time.Second*5)
}

func (o *videoSourceOperatorImpl) RecordKnownVideoSource(
	ctxt context.Context, id, name string, playlistURI, description *string, streaming int,
) error {
	return o.db.RecordKnownVideoSource(ctxt, id, name, playlistURI, description, streaming)
}

func (o *videoSourceOperatorImpl) ChangeVideoSourceStreamState(
	ctxt context.Context, id string, streaming int,
) error {
	return o.db.ChangeVideoSourceStreamState(ctxt, id, streaming)
}

func (o *videoSourceOperatorImpl) sendSourceStatusReport() error {
	logTags := o.GetLogTagsForContext(o.workerCtxt)

	report := ipc.NewVideoSourceStatusReport(o.self.ID, o.selfReqRespTargetID, time.Now().UTC())

	if err := o.reportStatus(o.workerCtxt, report); err != nil {
		log.WithError(err).WithFields(logTags).Error("Failed to send status report message")
		return err
	}

	return nil
}
