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

package engineimpl

import (
	ctx "context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/types"
	"github.com/uber/cadence/service/history/constants"
	"github.com/uber/cadence/service/history/engine/testdata"
	"github.com/uber/cadence/service/history/execution"
	"github.com/uber/cadence/service/history/reset"
)

var (
	testRequestID                = "this is a test request"
	testRequestReason            = "Test reason"
	testRequestSkipSignalReapply = true
	latestRunID                  = constants.TestRunID
	latestExecution              = &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: latestRunID}
	previousRunID                = "bbbbbeef-0123-4567-890a-bcdef0123456"
	previousExecution            = &types.WorkflowExecution{WorkflowID: constants.TestWorkflowID, RunID: previousRunID}
)

type InitFn func(t *testing.T, engine *testdata.EngineForTest)

func TestResetWorkflowExecution(t *testing.T) {
	cases := []struct {
		name        string
		request     *types.HistoryResetWorkflowExecutionRequest
		init        []InitFn
		expectedErr error
		expected    *types.ResetWorkflowExecutionResponse
	}{
		{
			name:    "No processed or pending decision",
			request: resetExecutionRequest(latestExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:           constants.TestDomainID,
						WorkflowID:         constants.TestWorkflowID,
						RunID:              latestRunID,
						DecisionScheduleID: common.EmptyEventID,
						LastProcessedEvent: common.EmptyEventID,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
			},
			expectedErr: &types.BadRequestError{Message: "Cannot reset workflow without a decision task schedule."},
		},
		{
			name:    "Invalid DecisionFinishEventId",
			request: resetExecutionRequest(latestExecution, 101),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 100,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
			},
			expectedErr: &types.BadRequestError{Message: "Decision finish ID must be > 1 && <= workflow next event ID."},
		},
		{
			name:    "Duplicate Request",
			request: resetExecutionRequest(latestExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:        constants.TestDomainID,
						WorkflowID:      constants.TestWorkflowID,
						RunID:           latestRunID,
						NextEventID:     100,
						CreateRequestID: testRequestID,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
			},
			expected: &types.ResetWorkflowExecutionResponse{
				RunID: latestRunID,
			},
		},
		{
			name:    "Success",
			request: resetExecutionRequest(latestExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 100,
						BranchToken: []byte("branch token"),
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: 1337,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq([]byte("branch token")),
						gomock.Eq(int64(99)),   // Request.DecisionFinishEventID - 1
						gomock.Eq(int64(1337)), // CurrentVersion
						gomock.Eq(int64(100)),  // NextEventID
						gomock.Any(),           // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(nil).Times(1)
				},
			},
			// Can't assert on the result because the runID is random
		},
		{
			name: "Success using version histories",
			// This corresponds to VersionHistories.Histories.Items.EventID
			request: resetExecutionRequest(latestExecution, 50),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 100,
						BranchToken: []byte("branch token"),
					},
					VersionHistories: &persistence.VersionHistories{
						CurrentVersionHistoryIndex: 1,
						Histories: []*persistence.VersionHistory{
							{
								BranchToken: []byte("this one isn't current"),
							},
							{
								BranchToken: []byte("yes"),
								Items: []*persistence.VersionHistoryItem{
									{
										EventID: 1,
										Version: 1001,
									},
									{
										EventID: 50,
										Version: 1002,
									},
									{
										EventID: 51,
										Version: 1003,
									},
								},
							},
						},
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq([]byte("yes")), //VersionHistories.Histories.BranchToken
						gomock.Eq(int64(49)),     // Request.DecisionFinishEventID - 1
						gomock.Eq(int64(1002)),   // VersionHistories.Histories.Items.Version
						gomock.Eq(int64(100)),    // NextEventID
						gomock.Any(),             // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(nil).Times(1)
				},
			},
			// Can't assert on the result because the runID is random
		},
		{
			name:    "Success using previous version",
			request: resetExecutionRequest(previousExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:   constants.TestDomainID,
						WorkflowID: constants.TestWorkflowID,
						RunID:      latestRunID,
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: 1,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				withState(previousExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 100,
						BranchToken: []byte("branch token for previous"),
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: 1337,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(previousExecution.RunID),
						gomock.Eq([]byte("branch token for previous")),
						gomock.Eq(int64(99)),   // Request.DecisionFinishEventID - 1
						gomock.Eq(int64(1337)), // CurrentVersion
						gomock.Eq(int64(100)),  // NextEventID
						gomock.Any(),           // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(nil).Times(1)
				},
			},
			// Can't assert on the result because the runID is random
		},
		{
			name:    "Persistence Duplicate Request",
			request: resetExecutionRequest(latestExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 100,
						BranchToken: []byte("branch token"),
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: 1337,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq([]byte("branch token")),
						gomock.Eq(int64(99)),   // Request.DecisionFinishEventID - 1
						gomock.Eq(int64(1337)), // CurrentVersion
						gomock.Eq(int64(100)),  // NextEventID
						gomock.Any(),           // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(&persistence.DuplicateRequestError{
						RequestType: persistence.WorkflowRequestTypeReset,
						RunID:       "errorID",
					}).Times(1)
				},
			},
			expected: &types.ResetWorkflowExecutionResponse{
				RunID: "errorID",
			},
		},
		{
			name:    "Persistence Duplicate Request Bug",
			request: resetExecutionRequest(latestExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 100,
						BranchToken: []byte("branch token"),
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: 1337,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq([]byte("branch token")),
						gomock.Eq(int64(99)),   // Request.DecisionFinishEventID - 1
						gomock.Eq(int64(1337)), // CurrentVersion
						gomock.Eq(int64(100)),  // NextEventID
						gomock.Any(),           // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(&persistence.DuplicateRequestError{
						RequestType: persistence.WorkflowRequestTypeStart,
						RunID:       "errorID",
					}).Times(1)
				},
			},
			expectedErr: &persistence.DuplicateRequestError{
				RequestType: persistence.WorkflowRequestTypeStart,
				RunID:       "errorID",
			},
		},
		{
			name:    "Reset returns Err",
			request: resetExecutionRequest(latestExecution, 100),
			init: []InitFn{
				withCurrentExecution(latestExecution),
				withState(latestExecution, &persistence.WorkflowMutableState{
					ExecutionInfo: &persistence.WorkflowExecutionInfo{
						DomainID:    constants.TestDomainID,
						WorkflowID:  constants.TestWorkflowID,
						RunID:       latestRunID,
						NextEventID: 100,
						BranchToken: []byte("branch token"),
					},
					ReplicationState: &persistence.ReplicationState{
						CurrentVersion: 1337,
					},
					ExecutionStats: &persistence.ExecutionStats{HistorySize: 1},
				}),
				func(t *testing.T, engine *testdata.EngineForTest) {
					ctrl := gomock.NewController(t)
					mockResetter := reset.NewMockWorkflowResetter(ctrl)
					engine.Engine.(*historyEngineImpl).workflowResetter = mockResetter

					mockResetter.EXPECT().ResetWorkflow(
						gomock.Any(), // Context
						gomock.Eq(constants.TestDomainID),
						gomock.Eq(constants.TestWorkflowID),
						gomock.Eq(latestExecution.RunID),
						gomock.Eq([]byte("branch token")),
						gomock.Eq(int64(99)),   // Request.DecisionFinishEventID - 1
						gomock.Eq(int64(1337)), // CurrentVersion
						gomock.Eq(int64(100)),  // NextEventID
						gomock.Any(),           // random uuid
						gomock.Eq(testRequestID),
						&workflowMatcher{latestExecution},
						gomock.Eq(testRequestReason),
						gomock.Nil(),
						gomock.Eq(testRequestSkipSignalReapply),
					).Return(&types.BadRequestError{
						Message: "didn't work",
					}).Times(1)
				},
			},
			expectedErr: &types.BadRequestError{
				Message: "didn't work",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			eft := testdata.NewEngineForTest(t, NewEngineWithShardContext)
			for _, setup := range testCase.init {
				setup(t, eft)
			}
			eft.Engine.Start()
			result, err := eft.Engine.ResetWorkflowExecution(ctx.Background(), testCase.request)
			eft.Engine.Stop()

			if testCase.expectedErr == nil {
				if assert.NotNil(t, result) {
					assert.NotEmpty(t, result.RunID)
				}
				assert.NoError(t, err)
			} else {
				assert.Nil(t, result)
				assert.Equal(t, testCase.expectedErr, err)
			}

		})
	}
}

func withCurrentExecution(execution *types.WorkflowExecution) InitFn {
	return func(_ *testing.T, engine *testdata.EngineForTest) {
		engine.ShardCtx.Resource.ExecutionMgr.On("GetCurrentExecution", mock.Anything, mock.Anything).Return(&persistence.GetCurrentExecutionResponse{
			StartRequestID: "CurrentExecutionStartRequestID",
			RunID:          execution.RunID,
			// Other fields don't matter
		}, nil)
	}
}

func withState(execution *types.WorkflowExecution, state *persistence.WorkflowMutableState) InitFn {
	return func(_ *testing.T, engine *testdata.EngineForTest) {
		engine.ShardCtx.Resource.ExecutionMgr.On("GetWorkflowExecution", mock.Anything, mock.MatchedBy(func(req *persistence.GetWorkflowExecutionRequest) bool {
			return req.Execution == *execution
		})).Return(&persistence.GetWorkflowExecutionResponse{
			State: state,
		}, nil)
	}
}

func resetExecutionRequest(execution *types.WorkflowExecution, decisionFinishEventID int) *types.HistoryResetWorkflowExecutionRequest {
	return &types.HistoryResetWorkflowExecutionRequest{
		DomainUUID: constants.TestDomainID,
		ResetRequest: &types.ResetWorkflowExecutionRequest{
			Domain:                constants.TestDomainName,
			WorkflowExecution:     execution,
			Reason:                testRequestReason,
			DecisionFinishEventID: int64(decisionFinishEventID),
			RequestID:             testRequestID,
			SkipSignalReapply:     testRequestSkipSignalReapply,
		},
	}
}

type workflowMatcher struct {
	execution *types.WorkflowExecution
}

func (m *workflowMatcher) Matches(obj interface{}) bool {
	if ex, ok := obj.(execution.Workflow); ok {
		executionInfo := ex.GetMutableState().GetExecutionInfo()
		return executionInfo.WorkflowID == m.execution.WorkflowID && executionInfo.RunID == m.execution.RunID
	}
	return false
}

func (m *workflowMatcher) String() string {
	return fmt.Sprintf("Workflow with WorkflowID %s and RunID %s", m.execution.WorkflowID, m.execution.RunID)
}
