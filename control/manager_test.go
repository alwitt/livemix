package control_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alwitt/livemix/common/ipc"
	"github.com/alwitt/livemix/control"
	"github.com/alwitt/livemix/mocks"
	"github.com/apex/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSystemManagerProcessSourceStatusBroadcast(t *testing.T) {
	assert := assert.New(t)
	log.SetLevel(log.DebugLevel)
	utCtxt := context.Background()

	mockDB := mocks.NewPersistenceManager(t)

	uut, err := control.NewManager(mockDB)
	assert.Nil(err)

	currentTime := time.Now().UTC()

	testMessage := ipc.NewVideoSourceStatusReport(
		uuid.NewString(), uuid.NewString(), currentTime,
	)

	broadcastMsg, err := json.Marshal(&testMessage)
	assert.Nil(err)

	// Setup mock
	mockDB.On(
		"RefreshVideoSourceStats",
		mock.AnythingOfType("*context.emptyCtx"),
		testMessage.SourceID,
		testMessage.RequestResponseTargetID,
		currentTime,
	).Return(nil).Once()

	// Process the broadcast message
	assert.Nil(uut.ProcessBroadcastMsgs(utCtxt, currentTime, broadcastMsg, nil))
}
