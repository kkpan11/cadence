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

package public

import (
	"context"
	"fmt"
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/assert"
)

// MockError to simulate gocql.Error behavior
type MockError struct {
	gocql.RequestError
	code    int
	message string
}

func (m MockError) Code() int {
	return m.code
}

func (m MockError) Message() string {
	return m.message
}

func TestClient_IsTimeoutError(t *testing.T) {
	client := client{}
	errorMap := map[error]bool{
		nil:                             false,
		context.DeadlineExceeded:        true,
		gocql.ErrTimeoutNoResponse:      true,
		gocql.ErrConnectionClosed:       true,
		&gocql.RequestErrWriteTimeout{}: true,
		gocql.ErrFrameTooBig:            false,
	}
	for err, expected := range errorMap {
		assert.Equal(t, expected, client.IsTimeoutError(err))
	}
}

func TestClient_IsNotFoundError(t *testing.T) {
	client := client{}
	errorMap := map[error]bool{
		nil:                  false,
		gocql.ErrNotFound:    true,
		gocql.ErrFrameTooBig: false,
	}
	for err, expected := range errorMap {
		assert.Equal(t, expected, client.IsNotFoundError(err))
	}
}

// TestClient_IsThrottlingError tests the IsThrottlingError function with different error codes
func TestClient_IsThrottlingError(t *testing.T) {
	client := client{}
	tests := []struct {
		name               string
		mockErrorCode      int
		expectedResult     bool
		nonCompatibleError error
	}{
		{
			name:           "With Throttling Error",
			mockErrorCode:  0x1001,
			expectedResult: true,
		},
		{
			name:               "With Non-Throttling Error",
			mockErrorCode:      0x0001,
			expectedResult:     false,
			nonCompatibleError: fmt.Errorf("with Non-Throttling Error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.nonCompatibleError != nil {
				result := client.IsThrottlingError(tt.nonCompatibleError)
				assert.False(t, result)
			}
			err := MockError{code: tt.mockErrorCode}
			result := client.IsThrottlingError(err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestClient_IsDBUnavailableError(t *testing.T) {
	client := client{}
	tests := []struct {
		name               string
		mockMessage        string
		mockErrorCode      int
		expectedResult     bool
		nonCompatibleError error
	}{
		{
			name:           "With DB Unavailable Error",
			mockMessage:    "Cannot perform LWT operation",
			mockErrorCode:  0x1000,
			expectedResult: true,
		},
		{
			name:           "With Non-DB Unavailable Error",
			mockMessage:    "Cannot perform LWT operation",
			mockErrorCode:  0x0001,
			expectedResult: false,
		},
		{
			name:               "With Non-compatible Error",
			mockErrorCode:      0x0001,
			expectedResult:     false,
			nonCompatibleError: fmt.Errorf("with Non-compatible Error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.nonCompatibleError != nil {
				result := client.IsDBUnavailableError(tt.nonCompatibleError)
				assert.False(t, result)
			}
			err := MockError{code: tt.mockErrorCode, message: tt.mockMessage}
			result := client.IsDBUnavailableError(err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestClient_IsCassandraConsistencyError(t *testing.T) {
	client := client{}
	tests := []struct {
		name               string
		mockErrorCode      int
		expectedResult     bool
		nonCompatibleError error
	}{
		{
			name:           "With Cassandra Consistency Error",
			mockErrorCode:  0x1000,
			expectedResult: true,
		},
		{
			name:           "With Non-Cassandra Consistency Error",
			mockErrorCode:  0x0001,
			expectedResult: false,
		},
		{
			name:               "With Non-compatible Error",
			mockErrorCode:      0x0001,
			expectedResult:     false,
			nonCompatibleError: fmt.Errorf("with Non-compatible Error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.nonCompatibleError != nil {
				result := client.IsCassandraConsistencyError(tt.nonCompatibleError)
				assert.False(t, result)
			}
			err := MockError{code: tt.mockErrorCode}
			result := client.IsCassandraConsistencyError(err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
