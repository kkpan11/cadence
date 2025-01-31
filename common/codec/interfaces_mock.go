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
// Source: github.com/uber/cadence/common/codec (interfaces: BinaryEncoder)
//
// Generated by this command:
//
//	mockgen -package codec -destination interfaces_mock.go -self_package github.com/uber/cadence/common/codec github.com/uber/cadence/common/codec BinaryEncoder
//

// Package codec is a generated GoMock package.
package codec

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockBinaryEncoder is a mock of BinaryEncoder interface.
type MockBinaryEncoder struct {
	ctrl     *gomock.Controller
	recorder *MockBinaryEncoderMockRecorder
	isgomock struct{}
}

// MockBinaryEncoderMockRecorder is the mock recorder for MockBinaryEncoder.
type MockBinaryEncoderMockRecorder struct {
	mock *MockBinaryEncoder
}

// NewMockBinaryEncoder creates a new mock instance.
func NewMockBinaryEncoder(ctrl *gomock.Controller) *MockBinaryEncoder {
	mock := &MockBinaryEncoder{ctrl: ctrl}
	mock.recorder = &MockBinaryEncoderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockBinaryEncoder) EXPECT() *MockBinaryEncoderMockRecorder {
	return m.recorder
}

// Decode mocks base method.
func (m *MockBinaryEncoder) Decode(payload []byte, val ThriftObject) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Decode", payload, val)
	ret0, _ := ret[0].(error)
	return ret0
}

// Decode indicates an expected call of Decode.
func (mr *MockBinaryEncoderMockRecorder) Decode(payload, val any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Decode", reflect.TypeOf((*MockBinaryEncoder)(nil).Decode), payload, val)
}

// Encode mocks base method.
func (m *MockBinaryEncoder) Encode(obj ThriftObject) ([]byte, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Encode", obj)
	ret0, _ := ret[0].([]byte)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Encode indicates an expected call of Encode.
func (mr *MockBinaryEncoderMockRecorder) Encode(obj any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Encode", reflect.TypeOf((*MockBinaryEncoder)(nil).Encode), obj)
}
