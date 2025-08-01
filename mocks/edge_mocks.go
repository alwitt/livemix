// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package mocks

import (
	"context"
	"time"

	"github.com/alwitt/livemix/common"
	"github.com/alwitt/livemix/edge"
	mock "github.com/stretchr/testify/mock"
)

// NewControlRequestClient creates a new instance of ControlRequestClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewControlRequestClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *ControlRequestClient {
	mock := &ControlRequestClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// ControlRequestClient is an autogenerated mock type for the ControlRequestClient type
type ControlRequestClient struct {
	mock.Mock
}

type ControlRequestClient_Expecter struct {
	mock *mock.Mock
}

func (_m *ControlRequestClient) EXPECT() *ControlRequestClient_Expecter {
	return &ControlRequestClient_Expecter{mock: &_m.Mock}
}

// ExchangeVideoSourceStatus provides a mock function for the type ControlRequestClient
func (_mock *ControlRequestClient) ExchangeVideoSourceStatus(ctxt context.Context, sourceID string, reqRespTargetID string, localTime time.Time) (common.VideoSource, []common.Recording, error) {
	ret := _mock.Called(ctxt, sourceID, reqRespTargetID, localTime)

	if len(ret) == 0 {
		panic("no return value specified for ExchangeVideoSourceStatus")
	}

	var r0 common.VideoSource
	var r1 []common.Recording
	var r2 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, string, string, time.Time) (common.VideoSource, []common.Recording, error)); ok {
		return returnFunc(ctxt, sourceID, reqRespTargetID, localTime)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context, string, string, time.Time) common.VideoSource); ok {
		r0 = returnFunc(ctxt, sourceID, reqRespTargetID, localTime)
	} else {
		r0 = ret.Get(0).(common.VideoSource)
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context, string, string, time.Time) []common.Recording); ok {
		r1 = returnFunc(ctxt, sourceID, reqRespTargetID, localTime)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]common.Recording)
		}
	}
	if returnFunc, ok := ret.Get(2).(func(context.Context, string, string, time.Time) error); ok {
		r2 = returnFunc(ctxt, sourceID, reqRespTargetID, localTime)
	} else {
		r2 = ret.Error(2)
	}
	return r0, r1, r2
}

// ControlRequestClient_ExchangeVideoSourceStatus_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ExchangeVideoSourceStatus'
type ControlRequestClient_ExchangeVideoSourceStatus_Call struct {
	*mock.Call
}

// ExchangeVideoSourceStatus is a helper method to define mock.On call
//   - ctxt context.Context
//   - sourceID string
//   - reqRespTargetID string
//   - localTime time.Time
func (_e *ControlRequestClient_Expecter) ExchangeVideoSourceStatus(ctxt interface{}, sourceID interface{}, reqRespTargetID interface{}, localTime interface{}) *ControlRequestClient_ExchangeVideoSourceStatus_Call {
	return &ControlRequestClient_ExchangeVideoSourceStatus_Call{Call: _e.mock.On("ExchangeVideoSourceStatus", ctxt, sourceID, reqRespTargetID, localTime)}
}

func (_c *ControlRequestClient_ExchangeVideoSourceStatus_Call) Run(run func(ctxt context.Context, sourceID string, reqRespTargetID string, localTime time.Time)) *ControlRequestClient_ExchangeVideoSourceStatus_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 string
		if args[1] != nil {
			arg1 = args[1].(string)
		}
		var arg2 string
		if args[2] != nil {
			arg2 = args[2].(string)
		}
		var arg3 time.Time
		if args[3] != nil {
			arg3 = args[3].(time.Time)
		}
		run(
			arg0,
			arg1,
			arg2,
			arg3,
		)
	})
	return _c
}

func (_c *ControlRequestClient_ExchangeVideoSourceStatus_Call) Return(videoSource common.VideoSource, recordings []common.Recording, err error) *ControlRequestClient_ExchangeVideoSourceStatus_Call {
	_c.Call.Return(videoSource, recordings, err)
	return _c
}

func (_c *ControlRequestClient_ExchangeVideoSourceStatus_Call) RunAndReturn(run func(ctxt context.Context, sourceID string, reqRespTargetID string, localTime time.Time) (common.VideoSource, []common.Recording, error)) *ControlRequestClient_ExchangeVideoSourceStatus_Call {
	_c.Call.Return(run)
	return _c
}

// GetVideoSourceInfo provides a mock function for the type ControlRequestClient
func (_mock *ControlRequestClient) GetVideoSourceInfo(ctxt context.Context, sourceName string) (common.VideoSource, error) {
	ret := _mock.Called(ctxt, sourceName)

	if len(ret) == 0 {
		panic("no return value specified for GetVideoSourceInfo")
	}

	var r0 common.VideoSource
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, string) (common.VideoSource, error)); ok {
		return returnFunc(ctxt, sourceName)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context, string) common.VideoSource); ok {
		r0 = returnFunc(ctxt, sourceName)
	} else {
		r0 = ret.Get(0).(common.VideoSource)
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = returnFunc(ctxt, sourceName)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// ControlRequestClient_GetVideoSourceInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetVideoSourceInfo'
type ControlRequestClient_GetVideoSourceInfo_Call struct {
	*mock.Call
}

// GetVideoSourceInfo is a helper method to define mock.On call
//   - ctxt context.Context
//   - sourceName string
func (_e *ControlRequestClient_Expecter) GetVideoSourceInfo(ctxt interface{}, sourceName interface{}) *ControlRequestClient_GetVideoSourceInfo_Call {
	return &ControlRequestClient_GetVideoSourceInfo_Call{Call: _e.mock.On("GetVideoSourceInfo", ctxt, sourceName)}
}

func (_c *ControlRequestClient_GetVideoSourceInfo_Call) Run(run func(ctxt context.Context, sourceName string)) *ControlRequestClient_GetVideoSourceInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 string
		if args[1] != nil {
			arg1 = args[1].(string)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *ControlRequestClient_GetVideoSourceInfo_Call) Return(videoSource common.VideoSource, err error) *ControlRequestClient_GetVideoSourceInfo_Call {
	_c.Call.Return(videoSource, err)
	return _c
}

func (_c *ControlRequestClient_GetVideoSourceInfo_Call) RunAndReturn(run func(ctxt context.Context, sourceName string) (common.VideoSource, error)) *ControlRequestClient_GetVideoSourceInfo_Call {
	_c.Call.Return(run)
	return _c
}

// InstallReferenceToManager provides a mock function for the type ControlRequestClient
func (_mock *ControlRequestClient) InstallReferenceToManager(newManager edge.VideoSourceOperator) {
	_mock.Called(newManager)
	return
}

// ControlRequestClient_InstallReferenceToManager_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'InstallReferenceToManager'
type ControlRequestClient_InstallReferenceToManager_Call struct {
	*mock.Call
}

// InstallReferenceToManager is a helper method to define mock.On call
//   - newManager edge.VideoSourceOperator
func (_e *ControlRequestClient_Expecter) InstallReferenceToManager(newManager interface{}) *ControlRequestClient_InstallReferenceToManager_Call {
	return &ControlRequestClient_InstallReferenceToManager_Call{Call: _e.mock.On("InstallReferenceToManager", newManager)}
}

func (_c *ControlRequestClient_InstallReferenceToManager_Call) Run(run func(newManager edge.VideoSourceOperator)) *ControlRequestClient_InstallReferenceToManager_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 edge.VideoSourceOperator
		if args[0] != nil {
			arg0 = args[0].(edge.VideoSourceOperator)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *ControlRequestClient_InstallReferenceToManager_Call) Return() *ControlRequestClient_InstallReferenceToManager_Call {
	_c.Call.Return()
	return _c
}

func (_c *ControlRequestClient_InstallReferenceToManager_Call) RunAndReturn(run func(newManager edge.VideoSourceOperator)) *ControlRequestClient_InstallReferenceToManager_Call {
	_c.Run(run)
	return _c
}

// ListActiveRecordingsOfSource provides a mock function for the type ControlRequestClient
func (_mock *ControlRequestClient) ListActiveRecordingsOfSource(ctxt context.Context, sourceID string) ([]common.Recording, error) {
	ret := _mock.Called(ctxt, sourceID)

	if len(ret) == 0 {
		panic("no return value specified for ListActiveRecordingsOfSource")
	}

	var r0 []common.Recording
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, string) ([]common.Recording, error)); ok {
		return returnFunc(ctxt, sourceID)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context, string) []common.Recording); ok {
		r0 = returnFunc(ctxt, sourceID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]common.Recording)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = returnFunc(ctxt, sourceID)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// ControlRequestClient_ListActiveRecordingsOfSource_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListActiveRecordingsOfSource'
type ControlRequestClient_ListActiveRecordingsOfSource_Call struct {
	*mock.Call
}

// ListActiveRecordingsOfSource is a helper method to define mock.On call
//   - ctxt context.Context
//   - sourceID string
func (_e *ControlRequestClient_Expecter) ListActiveRecordingsOfSource(ctxt interface{}, sourceID interface{}) *ControlRequestClient_ListActiveRecordingsOfSource_Call {
	return &ControlRequestClient_ListActiveRecordingsOfSource_Call{Call: _e.mock.On("ListActiveRecordingsOfSource", ctxt, sourceID)}
}

func (_c *ControlRequestClient_ListActiveRecordingsOfSource_Call) Run(run func(ctxt context.Context, sourceID string)) *ControlRequestClient_ListActiveRecordingsOfSource_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 string
		if args[1] != nil {
			arg1 = args[1].(string)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *ControlRequestClient_ListActiveRecordingsOfSource_Call) Return(recordings []common.Recording, err error) *ControlRequestClient_ListActiveRecordingsOfSource_Call {
	_c.Call.Return(recordings, err)
	return _c
}

func (_c *ControlRequestClient_ListActiveRecordingsOfSource_Call) RunAndReturn(run func(ctxt context.Context, sourceID string) ([]common.Recording, error)) *ControlRequestClient_ListActiveRecordingsOfSource_Call {
	_c.Call.Return(run)
	return _c
}

// StopAllAssociatedRecordings provides a mock function for the type ControlRequestClient
func (_mock *ControlRequestClient) StopAllAssociatedRecordings(ctxt context.Context, sourceID string) error {
	ret := _mock.Called(ctxt, sourceID)

	if len(ret) == 0 {
		panic("no return value specified for StopAllAssociatedRecordings")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = returnFunc(ctxt, sourceID)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// ControlRequestClient_StopAllAssociatedRecordings_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'StopAllAssociatedRecordings'
type ControlRequestClient_StopAllAssociatedRecordings_Call struct {
	*mock.Call
}

// StopAllAssociatedRecordings is a helper method to define mock.On call
//   - ctxt context.Context
//   - sourceID string
func (_e *ControlRequestClient_Expecter) StopAllAssociatedRecordings(ctxt interface{}, sourceID interface{}) *ControlRequestClient_StopAllAssociatedRecordings_Call {
	return &ControlRequestClient_StopAllAssociatedRecordings_Call{Call: _e.mock.On("StopAllAssociatedRecordings", ctxt, sourceID)}
}

func (_c *ControlRequestClient_StopAllAssociatedRecordings_Call) Run(run func(ctxt context.Context, sourceID string)) *ControlRequestClient_StopAllAssociatedRecordings_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 string
		if args[1] != nil {
			arg1 = args[1].(string)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *ControlRequestClient_StopAllAssociatedRecordings_Call) Return(err error) *ControlRequestClient_StopAllAssociatedRecordings_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *ControlRequestClient_StopAllAssociatedRecordings_Call) RunAndReturn(run func(ctxt context.Context, sourceID string) error) *ControlRequestClient_StopAllAssociatedRecordings_Call {
	_c.Call.Return(run)
	return _c
}

// NewVideoSourceOperator creates a new instance of VideoSourceOperator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewVideoSourceOperator(t interface {
	mock.TestingT
	Cleanup(func())
}) *VideoSourceOperator {
	mock := &VideoSourceOperator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// VideoSourceOperator is an autogenerated mock type for the VideoSourceOperator type
type VideoSourceOperator struct {
	mock.Mock
}

type VideoSourceOperator_Expecter struct {
	mock *mock.Mock
}

func (_m *VideoSourceOperator) EXPECT() *VideoSourceOperator_Expecter {
	return &VideoSourceOperator_Expecter{mock: &_m.Mock}
}

// CacheEntryCount provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) CacheEntryCount(ctxt context.Context) (int, error) {
	ret := _mock.Called(ctxt)

	if len(ret) == 0 {
		panic("no return value specified for CacheEntryCount")
	}

	var r0 int
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(context.Context) (int, error)); ok {
		return returnFunc(ctxt)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context) int); ok {
		r0 = returnFunc(ctxt)
	} else {
		r0 = ret.Get(0).(int)
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = returnFunc(ctxt)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// VideoSourceOperator_CacheEntryCount_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CacheEntryCount'
type VideoSourceOperator_CacheEntryCount_Call struct {
	*mock.Call
}

// CacheEntryCount is a helper method to define mock.On call
//   - ctxt context.Context
func (_e *VideoSourceOperator_Expecter) CacheEntryCount(ctxt interface{}) *VideoSourceOperator_CacheEntryCount_Call {
	return &VideoSourceOperator_CacheEntryCount_Call{Call: _e.mock.On("CacheEntryCount", ctxt)}
}

func (_c *VideoSourceOperator_CacheEntryCount_Call) Run(run func(ctxt context.Context)) *VideoSourceOperator_CacheEntryCount_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_CacheEntryCount_Call) Return(n int, err error) *VideoSourceOperator_CacheEntryCount_Call {
	_c.Call.Return(n, err)
	return _c
}

func (_c *VideoSourceOperator_CacheEntryCount_Call) RunAndReturn(run func(ctxt context.Context) (int, error)) *VideoSourceOperator_CacheEntryCount_Call {
	_c.Call.Return(run)
	return _c
}

// ChangeVideoSourceStreamState provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) ChangeVideoSourceStreamState(ctxt context.Context, id string, streaming int) error {
	ret := _mock.Called(ctxt, id, streaming)

	if len(ret) == 0 {
		panic("no return value specified for ChangeVideoSourceStreamState")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, string, int) error); ok {
		r0 = returnFunc(ctxt, id, streaming)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// VideoSourceOperator_ChangeVideoSourceStreamState_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ChangeVideoSourceStreamState'
type VideoSourceOperator_ChangeVideoSourceStreamState_Call struct {
	*mock.Call
}

// ChangeVideoSourceStreamState is a helper method to define mock.On call
//   - ctxt context.Context
//   - id string
//   - streaming int
func (_e *VideoSourceOperator_Expecter) ChangeVideoSourceStreamState(ctxt interface{}, id interface{}, streaming interface{}) *VideoSourceOperator_ChangeVideoSourceStreamState_Call {
	return &VideoSourceOperator_ChangeVideoSourceStreamState_Call{Call: _e.mock.On("ChangeVideoSourceStreamState", ctxt, id, streaming)}
}

func (_c *VideoSourceOperator_ChangeVideoSourceStreamState_Call) Run(run func(ctxt context.Context, id string, streaming int)) *VideoSourceOperator_ChangeVideoSourceStreamState_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 string
		if args[1] != nil {
			arg1 = args[1].(string)
		}
		var arg2 int
		if args[2] != nil {
			arg2 = args[2].(int)
		}
		run(
			arg0,
			arg1,
			arg2,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_ChangeVideoSourceStreamState_Call) Return(err error) *VideoSourceOperator_ChangeVideoSourceStreamState_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *VideoSourceOperator_ChangeVideoSourceStreamState_Call) RunAndReturn(run func(ctxt context.Context, id string, streaming int) error) *VideoSourceOperator_ChangeVideoSourceStreamState_Call {
	_c.Call.Return(run)
	return _c
}

// NewSegmentFromSource provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) NewSegmentFromSource(ctxt context.Context, segment common.VideoSegmentWithData) error {
	ret := _mock.Called(ctxt, segment)

	if len(ret) == 0 {
		panic("no return value specified for NewSegmentFromSource")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, common.VideoSegmentWithData) error); ok {
		r0 = returnFunc(ctxt, segment)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// VideoSourceOperator_NewSegmentFromSource_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NewSegmentFromSource'
type VideoSourceOperator_NewSegmentFromSource_Call struct {
	*mock.Call
}

// NewSegmentFromSource is a helper method to define mock.On call
//   - ctxt context.Context
//   - segment common.VideoSegmentWithData
func (_e *VideoSourceOperator_Expecter) NewSegmentFromSource(ctxt interface{}, segment interface{}) *VideoSourceOperator_NewSegmentFromSource_Call {
	return &VideoSourceOperator_NewSegmentFromSource_Call{Call: _e.mock.On("NewSegmentFromSource", ctxt, segment)}
}

func (_c *VideoSourceOperator_NewSegmentFromSource_Call) Run(run func(ctxt context.Context, segment common.VideoSegmentWithData)) *VideoSourceOperator_NewSegmentFromSource_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 common.VideoSegmentWithData
		if args[1] != nil {
			arg1 = args[1].(common.VideoSegmentWithData)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_NewSegmentFromSource_Call) Return(err error) *VideoSourceOperator_NewSegmentFromSource_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *VideoSourceOperator_NewSegmentFromSource_Call) RunAndReturn(run func(ctxt context.Context, segment common.VideoSegmentWithData) error) *VideoSourceOperator_NewSegmentFromSource_Call {
	_c.Call.Return(run)
	return _c
}

// Ready provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) Ready(ctxt context.Context) error {
	ret := _mock.Called(ctxt)

	if len(ret) == 0 {
		panic("no return value specified for Ready")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = returnFunc(ctxt)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// VideoSourceOperator_Ready_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Ready'
type VideoSourceOperator_Ready_Call struct {
	*mock.Call
}

// Ready is a helper method to define mock.On call
//   - ctxt context.Context
func (_e *VideoSourceOperator_Expecter) Ready(ctxt interface{}) *VideoSourceOperator_Ready_Call {
	return &VideoSourceOperator_Ready_Call{Call: _e.mock.On("Ready", ctxt)}
}

func (_c *VideoSourceOperator_Ready_Call) Run(run func(ctxt context.Context)) *VideoSourceOperator_Ready_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_Ready_Call) Return(err error) *VideoSourceOperator_Ready_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *VideoSourceOperator_Ready_Call) RunAndReturn(run func(ctxt context.Context) error) *VideoSourceOperator_Ready_Call {
	_c.Call.Return(run)
	return _c
}

// RecordKnownVideoSource provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) RecordKnownVideoSource(ctxt context.Context, id string, name string, segmentLen int, playlistURI *string, description *string, streaming int) error {
	ret := _mock.Called(ctxt, id, name, segmentLen, playlistURI, description, streaming)

	if len(ret) == 0 {
		panic("no return value specified for RecordKnownVideoSource")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, string, string, int, *string, *string, int) error); ok {
		r0 = returnFunc(ctxt, id, name, segmentLen, playlistURI, description, streaming)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// VideoSourceOperator_RecordKnownVideoSource_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RecordKnownVideoSource'
type VideoSourceOperator_RecordKnownVideoSource_Call struct {
	*mock.Call
}

// RecordKnownVideoSource is a helper method to define mock.On call
//   - ctxt context.Context
//   - id string
//   - name string
//   - segmentLen int
//   - playlistURI *string
//   - description *string
//   - streaming int
func (_e *VideoSourceOperator_Expecter) RecordKnownVideoSource(ctxt interface{}, id interface{}, name interface{}, segmentLen interface{}, playlistURI interface{}, description interface{}, streaming interface{}) *VideoSourceOperator_RecordKnownVideoSource_Call {
	return &VideoSourceOperator_RecordKnownVideoSource_Call{Call: _e.mock.On("RecordKnownVideoSource", ctxt, id, name, segmentLen, playlistURI, description, streaming)}
}

func (_c *VideoSourceOperator_RecordKnownVideoSource_Call) Run(run func(ctxt context.Context, id string, name string, segmentLen int, playlistURI *string, description *string, streaming int)) *VideoSourceOperator_RecordKnownVideoSource_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 string
		if args[1] != nil {
			arg1 = args[1].(string)
		}
		var arg2 string
		if args[2] != nil {
			arg2 = args[2].(string)
		}
		var arg3 int
		if args[3] != nil {
			arg3 = args[3].(int)
		}
		var arg4 *string
		if args[4] != nil {
			arg4 = args[4].(*string)
		}
		var arg5 *string
		if args[5] != nil {
			arg5 = args[5].(*string)
		}
		var arg6 int
		if args[6] != nil {
			arg6 = args[6].(int)
		}
		run(
			arg0,
			arg1,
			arg2,
			arg3,
			arg4,
			arg5,
			arg6,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_RecordKnownVideoSource_Call) Return(err error) *VideoSourceOperator_RecordKnownVideoSource_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *VideoSourceOperator_RecordKnownVideoSource_Call) RunAndReturn(run func(ctxt context.Context, id string, name string, segmentLen int, playlistURI *string, description *string, streaming int) error) *VideoSourceOperator_RecordKnownVideoSource_Call {
	_c.Call.Return(run)
	return _c
}

// StartRecording provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) StartRecording(ctxt context.Context, newRecording common.Recording) error {
	ret := _mock.Called(ctxt, newRecording)

	if len(ret) == 0 {
		panic("no return value specified for StartRecording")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, common.Recording) error); ok {
		r0 = returnFunc(ctxt, newRecording)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// VideoSourceOperator_StartRecording_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'StartRecording'
type VideoSourceOperator_StartRecording_Call struct {
	*mock.Call
}

// StartRecording is a helper method to define mock.On call
//   - ctxt context.Context
//   - newRecording common.Recording
func (_e *VideoSourceOperator_Expecter) StartRecording(ctxt interface{}, newRecording interface{}) *VideoSourceOperator_StartRecording_Call {
	return &VideoSourceOperator_StartRecording_Call{Call: _e.mock.On("StartRecording", ctxt, newRecording)}
}

func (_c *VideoSourceOperator_StartRecording_Call) Run(run func(ctxt context.Context, newRecording common.Recording)) *VideoSourceOperator_StartRecording_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 common.Recording
		if args[1] != nil {
			arg1 = args[1].(common.Recording)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_StartRecording_Call) Return(err error) *VideoSourceOperator_StartRecording_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *VideoSourceOperator_StartRecording_Call) RunAndReturn(run func(ctxt context.Context, newRecording common.Recording) error) *VideoSourceOperator_StartRecording_Call {
	_c.Call.Return(run)
	return _c
}

// Stop provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) Stop(ctxt context.Context) error {
	ret := _mock.Called(ctxt)

	if len(ret) == 0 {
		panic("no return value specified for Stop")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = returnFunc(ctxt)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// VideoSourceOperator_Stop_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Stop'
type VideoSourceOperator_Stop_Call struct {
	*mock.Call
}

// Stop is a helper method to define mock.On call
//   - ctxt context.Context
func (_e *VideoSourceOperator_Expecter) Stop(ctxt interface{}) *VideoSourceOperator_Stop_Call {
	return &VideoSourceOperator_Stop_Call{Call: _e.mock.On("Stop", ctxt)}
}

func (_c *VideoSourceOperator_Stop_Call) Run(run func(ctxt context.Context)) *VideoSourceOperator_Stop_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_Stop_Call) Return(err error) *VideoSourceOperator_Stop_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *VideoSourceOperator_Stop_Call) RunAndReturn(run func(ctxt context.Context) error) *VideoSourceOperator_Stop_Call {
	_c.Call.Return(run)
	return _c
}

// StopRecording provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) StopRecording(ctxt context.Context, recordingID string, endTime time.Time) error {
	ret := _mock.Called(ctxt, recordingID, endTime)

	if len(ret) == 0 {
		panic("no return value specified for StopRecording")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, string, time.Time) error); ok {
		r0 = returnFunc(ctxt, recordingID, endTime)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// VideoSourceOperator_StopRecording_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'StopRecording'
type VideoSourceOperator_StopRecording_Call struct {
	*mock.Call
}

// StopRecording is a helper method to define mock.On call
//   - ctxt context.Context
//   - recordingID string
//   - endTime time.Time
func (_e *VideoSourceOperator_Expecter) StopRecording(ctxt interface{}, recordingID interface{}, endTime interface{}) *VideoSourceOperator_StopRecording_Call {
	return &VideoSourceOperator_StopRecording_Call{Call: _e.mock.On("StopRecording", ctxt, recordingID, endTime)}
}

func (_c *VideoSourceOperator_StopRecording_Call) Run(run func(ctxt context.Context, recordingID string, endTime time.Time)) *VideoSourceOperator_StopRecording_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 string
		if args[1] != nil {
			arg1 = args[1].(string)
		}
		var arg2 time.Time
		if args[2] != nil {
			arg2 = args[2].(time.Time)
		}
		run(
			arg0,
			arg1,
			arg2,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_StopRecording_Call) Return(err error) *VideoSourceOperator_StopRecording_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *VideoSourceOperator_StopRecording_Call) RunAndReturn(run func(ctxt context.Context, recordingID string, endTime time.Time) error) *VideoSourceOperator_StopRecording_Call {
	_c.Call.Return(run)
	return _c
}

// SyncActiveRecordingState provides a mock function for the type VideoSourceOperator
func (_mock *VideoSourceOperator) SyncActiveRecordingState(timestamp time.Time) error {
	ret := _mock.Called(timestamp)

	if len(ret) == 0 {
		panic("no return value specified for SyncActiveRecordingState")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(time.Time) error); ok {
		r0 = returnFunc(timestamp)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// VideoSourceOperator_SyncActiveRecordingState_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SyncActiveRecordingState'
type VideoSourceOperator_SyncActiveRecordingState_Call struct {
	*mock.Call
}

// SyncActiveRecordingState is a helper method to define mock.On call
//   - timestamp time.Time
func (_e *VideoSourceOperator_Expecter) SyncActiveRecordingState(timestamp interface{}) *VideoSourceOperator_SyncActiveRecordingState_Call {
	return &VideoSourceOperator_SyncActiveRecordingState_Call{Call: _e.mock.On("SyncActiveRecordingState", timestamp)}
}

func (_c *VideoSourceOperator_SyncActiveRecordingState_Call) Run(run func(timestamp time.Time)) *VideoSourceOperator_SyncActiveRecordingState_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 time.Time
		if args[0] != nil {
			arg0 = args[0].(time.Time)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *VideoSourceOperator_SyncActiveRecordingState_Call) Return(err error) *VideoSourceOperator_SyncActiveRecordingState_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *VideoSourceOperator_SyncActiveRecordingState_Call) RunAndReturn(run func(timestamp time.Time) error) *VideoSourceOperator_SyncActiveRecordingState_Call {
	_c.Call.Return(run)
	return _c
}
