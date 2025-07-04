// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package mocks

import (
	"context"

	"github.com/alwitt/livemix/common"
	mock "github.com/stretchr/testify/mock"
)

// NewLiveStreamSegmentForwarder creates a new instance of LiveStreamSegmentForwarder. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewLiveStreamSegmentForwarder(t interface {
	mock.TestingT
	Cleanup(func())
}) *LiveStreamSegmentForwarder {
	mock := &LiveStreamSegmentForwarder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// LiveStreamSegmentForwarder is an autogenerated mock type for the LiveStreamSegmentForwarder type
type LiveStreamSegmentForwarder struct {
	mock.Mock
}

type LiveStreamSegmentForwarder_Expecter struct {
	mock *mock.Mock
}

func (_m *LiveStreamSegmentForwarder) EXPECT() *LiveStreamSegmentForwarder_Expecter {
	return &LiveStreamSegmentForwarder_Expecter{mock: &_m.Mock}
}

// ForwardSegment provides a mock function for the type LiveStreamSegmentForwarder
func (_mock *LiveStreamSegmentForwarder) ForwardSegment(ctxt context.Context, segment common.VideoSegmentWithData, blocking bool) error {
	ret := _mock.Called(ctxt, segment, blocking)

	if len(ret) == 0 {
		panic("no return value specified for ForwardSegment")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, common.VideoSegmentWithData, bool) error); ok {
		r0 = returnFunc(ctxt, segment, blocking)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// LiveStreamSegmentForwarder_ForwardSegment_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ForwardSegment'
type LiveStreamSegmentForwarder_ForwardSegment_Call struct {
	*mock.Call
}

// ForwardSegment is a helper method to define mock.On call
//   - ctxt context.Context
//   - segment common.VideoSegmentWithData
//   - blocking bool
func (_e *LiveStreamSegmentForwarder_Expecter) ForwardSegment(ctxt interface{}, segment interface{}, blocking interface{}) *LiveStreamSegmentForwarder_ForwardSegment_Call {
	return &LiveStreamSegmentForwarder_ForwardSegment_Call{Call: _e.mock.On("ForwardSegment", ctxt, segment, blocking)}
}

func (_c *LiveStreamSegmentForwarder_ForwardSegment_Call) Run(run func(ctxt context.Context, segment common.VideoSegmentWithData, blocking bool)) *LiveStreamSegmentForwarder_ForwardSegment_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 common.VideoSegmentWithData
		if args[1] != nil {
			arg1 = args[1].(common.VideoSegmentWithData)
		}
		var arg2 bool
		if args[2] != nil {
			arg2 = args[2].(bool)
		}
		run(
			arg0,
			arg1,
			arg2,
		)
	})
	return _c
}

func (_c *LiveStreamSegmentForwarder_ForwardSegment_Call) Return(err error) *LiveStreamSegmentForwarder_ForwardSegment_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *LiveStreamSegmentForwarder_ForwardSegment_Call) RunAndReturn(run func(ctxt context.Context, segment common.VideoSegmentWithData, blocking bool) error) *LiveStreamSegmentForwarder_ForwardSegment_Call {
	_c.Call.Return(run)
	return _c
}

// Stop provides a mock function for the type LiveStreamSegmentForwarder
func (_mock *LiveStreamSegmentForwarder) Stop(ctxt context.Context) error {
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

// LiveStreamSegmentForwarder_Stop_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Stop'
type LiveStreamSegmentForwarder_Stop_Call struct {
	*mock.Call
}

// Stop is a helper method to define mock.On call
//   - ctxt context.Context
func (_e *LiveStreamSegmentForwarder_Expecter) Stop(ctxt interface{}) *LiveStreamSegmentForwarder_Stop_Call {
	return &LiveStreamSegmentForwarder_Stop_Call{Call: _e.mock.On("Stop", ctxt)}
}

func (_c *LiveStreamSegmentForwarder_Stop_Call) Run(run func(ctxt context.Context)) *LiveStreamSegmentForwarder_Stop_Call {
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

func (_c *LiveStreamSegmentForwarder_Stop_Call) Return(err error) *LiveStreamSegmentForwarder_Stop_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *LiveStreamSegmentForwarder_Stop_Call) RunAndReturn(run func(ctxt context.Context) error) *LiveStreamSegmentForwarder_Stop_Call {
	_c.Call.Return(run)
	return _c
}

// NewRecordingSegmentForwarder creates a new instance of RecordingSegmentForwarder. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRecordingSegmentForwarder(t interface {
	mock.TestingT
	Cleanup(func())
}) *RecordingSegmentForwarder {
	mock := &RecordingSegmentForwarder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// RecordingSegmentForwarder is an autogenerated mock type for the RecordingSegmentForwarder type
type RecordingSegmentForwarder struct {
	mock.Mock
}

type RecordingSegmentForwarder_Expecter struct {
	mock *mock.Mock
}

func (_m *RecordingSegmentForwarder) EXPECT() *RecordingSegmentForwarder_Expecter {
	return &RecordingSegmentForwarder_Expecter{mock: &_m.Mock}
}

// ForwardSegment provides a mock function for the type RecordingSegmentForwarder
func (_mock *RecordingSegmentForwarder) ForwardSegment(ctxt context.Context, recordings []string, segments []common.VideoSegmentWithData) error {
	ret := _mock.Called(ctxt, recordings, segments)

	if len(ret) == 0 {
		panic("no return value specified for ForwardSegment")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, []string, []common.VideoSegmentWithData) error); ok {
		r0 = returnFunc(ctxt, recordings, segments)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// RecordingSegmentForwarder_ForwardSegment_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ForwardSegment'
type RecordingSegmentForwarder_ForwardSegment_Call struct {
	*mock.Call
}

// ForwardSegment is a helper method to define mock.On call
//   - ctxt context.Context
//   - recordings []string
//   - segments []common.VideoSegmentWithData
func (_e *RecordingSegmentForwarder_Expecter) ForwardSegment(ctxt interface{}, recordings interface{}, segments interface{}) *RecordingSegmentForwarder_ForwardSegment_Call {
	return &RecordingSegmentForwarder_ForwardSegment_Call{Call: _e.mock.On("ForwardSegment", ctxt, recordings, segments)}
}

func (_c *RecordingSegmentForwarder_ForwardSegment_Call) Run(run func(ctxt context.Context, recordings []string, segments []common.VideoSegmentWithData)) *RecordingSegmentForwarder_ForwardSegment_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 []string
		if args[1] != nil {
			arg1 = args[1].([]string)
		}
		var arg2 []common.VideoSegmentWithData
		if args[2] != nil {
			arg2 = args[2].([]common.VideoSegmentWithData)
		}
		run(
			arg0,
			arg1,
			arg2,
		)
	})
	return _c
}

func (_c *RecordingSegmentForwarder_ForwardSegment_Call) Return(err error) *RecordingSegmentForwarder_ForwardSegment_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *RecordingSegmentForwarder_ForwardSegment_Call) RunAndReturn(run func(ctxt context.Context, recordings []string, segments []common.VideoSegmentWithData) error) *RecordingSegmentForwarder_ForwardSegment_Call {
	_c.Call.Return(run)
	return _c
}

// Stop provides a mock function for the type RecordingSegmentForwarder
func (_mock *RecordingSegmentForwarder) Stop(ctxt context.Context) error {
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

// RecordingSegmentForwarder_Stop_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Stop'
type RecordingSegmentForwarder_Stop_Call struct {
	*mock.Call
}

// Stop is a helper method to define mock.On call
//   - ctxt context.Context
func (_e *RecordingSegmentForwarder_Expecter) Stop(ctxt interface{}) *RecordingSegmentForwarder_Stop_Call {
	return &RecordingSegmentForwarder_Stop_Call{Call: _e.mock.On("Stop", ctxt)}
}

func (_c *RecordingSegmentForwarder_Stop_Call) Run(run func(ctxt context.Context)) *RecordingSegmentForwarder_Stop_Call {
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

func (_c *RecordingSegmentForwarder_Stop_Call) Return(err error) *RecordingSegmentForwarder_Stop_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *RecordingSegmentForwarder_Stop_Call) RunAndReturn(run func(ctxt context.Context) error) *RecordingSegmentForwarder_Stop_Call {
	_c.Call.Return(run)
	return _c
}

// NewSegmentSender creates a new instance of SegmentSender. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSegmentSender(t interface {
	mock.TestingT
	Cleanup(func())
}) *SegmentSender {
	mock := &SegmentSender{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// SegmentSender is an autogenerated mock type for the SegmentSender type
type SegmentSender struct {
	mock.Mock
}

type SegmentSender_Expecter struct {
	mock *mock.Mock
}

func (_m *SegmentSender) EXPECT() *SegmentSender_Expecter {
	return &SegmentSender_Expecter{mock: &_m.Mock}
}

// ForwardSegment provides a mock function for the type SegmentSender
func (_mock *SegmentSender) ForwardSegment(ctxt context.Context, segment common.VideoSegmentWithData) error {
	ret := _mock.Called(ctxt, segment)

	if len(ret) == 0 {
		panic("no return value specified for ForwardSegment")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, common.VideoSegmentWithData) error); ok {
		r0 = returnFunc(ctxt, segment)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// SegmentSender_ForwardSegment_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ForwardSegment'
type SegmentSender_ForwardSegment_Call struct {
	*mock.Call
}

// ForwardSegment is a helper method to define mock.On call
//   - ctxt context.Context
//   - segment common.VideoSegmentWithData
func (_e *SegmentSender_Expecter) ForwardSegment(ctxt interface{}, segment interface{}) *SegmentSender_ForwardSegment_Call {
	return &SegmentSender_ForwardSegment_Call{Call: _e.mock.On("ForwardSegment", ctxt, segment)}
}

func (_c *SegmentSender_ForwardSegment_Call) Run(run func(ctxt context.Context, segment common.VideoSegmentWithData)) *SegmentSender_ForwardSegment_Call {
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

func (_c *SegmentSender_ForwardSegment_Call) Return(err error) *SegmentSender_ForwardSegment_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *SegmentSender_ForwardSegment_Call) RunAndReturn(run func(ctxt context.Context, segment common.VideoSegmentWithData) error) *SegmentSender_ForwardSegment_Call {
	_c.Call.Return(run)
	return _c
}
