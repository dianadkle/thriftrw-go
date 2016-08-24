// Automatically generated by MockGen. DO NOT EDIT!
// Source: io (interfaces: Reader,ReadCloser)

package frame

import (
	gomock "github.com/golang/mock/gomock"
)

// Mock of Reader interface
type MockReader struct {
	ctrl     *gomock.Controller
	recorder *_MockReaderRecorder
}

// Recorder for MockReader (not exported)
type _MockReaderRecorder struct {
	mock *MockReader
}

func NewMockReader(ctrl *gomock.Controller) *MockReader {
	mock := &MockReader{ctrl: ctrl}
	mock.recorder = &_MockReaderRecorder{mock}
	return mock
}

func (_m *MockReader) EXPECT() *_MockReaderRecorder {
	return _m.recorder
}

func (_m *MockReader) Read(_param0 []byte) (int, error) {
	ret := _m.ctrl.Call(_m, "Read", _param0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockReaderRecorder) Read(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Read", arg0)
}

// Mock of ReadCloser interface
type MockReadCloser struct {
	ctrl     *gomock.Controller
	recorder *_MockReadCloserRecorder
}

// Recorder for MockReadCloser (not exported)
type _MockReadCloserRecorder struct {
	mock *MockReadCloser
}

func NewMockReadCloser(ctrl *gomock.Controller) *MockReadCloser {
	mock := &MockReadCloser{ctrl: ctrl}
	mock.recorder = &_MockReadCloserRecorder{mock}
	return mock
}

func (_m *MockReadCloser) EXPECT() *_MockReadCloserRecorder {
	return _m.recorder
}

func (_m *MockReadCloser) Close() error {
	ret := _m.ctrl.Call(_m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockReadCloserRecorder) Close() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Close")
}

func (_m *MockReadCloser) Read(_param0 []byte) (int, error) {
	ret := _m.ctrl.Call(_m, "Read", _param0)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockReadCloserRecorder) Read(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Read", arg0)
}