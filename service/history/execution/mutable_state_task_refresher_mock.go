// The MIT License (MIT)

// Copyright (c) 2017-2020 Uber Technologies Inc.

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Code generated by MockGen. DO NOT EDIT.
// Source: mutable_state_task_refresher.go
//
// Generated by this command:
//
//	mockgen -package execution -source mutable_state_task_refresher.go -destination mutable_state_task_refresher_mock.go -self_package github.com/uber/cadence/service/history/execution
//

// Package execution is a generated GoMock package.
package execution

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "go.uber.org/mock/gomock"
)

// MockMutableStateTaskRefresher is a mock of MutableStateTaskRefresher interface.
type MockMutableStateTaskRefresher struct {
	ctrl     *gomock.Controller
	recorder *MockMutableStateTaskRefresherMockRecorder
	isgomock struct{}
}

// MockMutableStateTaskRefresherMockRecorder is the mock recorder for MockMutableStateTaskRefresher.
type MockMutableStateTaskRefresherMockRecorder struct {
	mock *MockMutableStateTaskRefresher
}

// NewMockMutableStateTaskRefresher creates a new mock instance.
func NewMockMutableStateTaskRefresher(ctrl *gomock.Controller) *MockMutableStateTaskRefresher {
	mock := &MockMutableStateTaskRefresher{ctrl: ctrl}
	mock.recorder = &MockMutableStateTaskRefresherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMutableStateTaskRefresher) EXPECT() *MockMutableStateTaskRefresherMockRecorder {
	return m.recorder
}

// RefreshTasks mocks base method.
func (m *MockMutableStateTaskRefresher) RefreshTasks(ctx context.Context, startTime time.Time, mutableState MutableState) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RefreshTasks", ctx, startTime, mutableState)
	ret0, _ := ret[0].(error)
	return ret0
}

// RefreshTasks indicates an expected call of RefreshTasks.
func (mr *MockMutableStateTaskRefresherMockRecorder) RefreshTasks(ctx, startTime, mutableState any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RefreshTasks", reflect.TypeOf((*MockMutableStateTaskRefresher)(nil).RefreshTasks), ctx, startTime, mutableState)
}
