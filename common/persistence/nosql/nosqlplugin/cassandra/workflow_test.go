// Copyright (c) 2021 Uber Technologies, Inc.
// Portions of the Software are attributed to Copyright (c) 2020 Temporal Technologies Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cassandra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	"github.com/uber/cadence/common"
	"github.com/uber/cadence/common/config"
	"github.com/uber/cadence/common/log/testlogger"
	"github.com/uber/cadence/common/persistence"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/gocql"
	"github.com/uber/cadence/common/persistence/nosql/nosqlplugin/cassandra/testdata"
)

func TestInsertWorkflowExecutionWithTasks(t *testing.T) {
	tests := []struct {
		name                  string
		workflowRequest       *nosqlplugin.WorkflowRequestsWriteRequest
		request               *nosqlplugin.CurrentWorkflowWriteRequest
		execution             *nosqlplugin.WorkflowExecutionRequest
		transferTasks         []*nosqlplugin.TransferTask
		crossClusterTasks     []*nosqlplugin.CrossClusterTask
		replicationTasks      []*nosqlplugin.ReplicationTask
		timerTasks            []*nosqlplugin.TimerTask
		shardCondition        *nosqlplugin.ShardCondition
		mapExecuteBatchCASErr error
		wantErr               bool
	}{
		{
			name: "success",
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			execution: testdata.WFExecRequest(),
		},
		{
			name: "insertOrUpsertWorkflowRequestRow step fails",
			workflowRequest: &nosqlplugin.WorkflowRequestsWriteRequest{
				Rows: []*nosqlplugin.WorkflowRequestRow{
					{
						RequestType: persistence.WorkflowRequestTypeStart,
					},
				},
				WriteMode: nosqlplugin.WorkflowRequestWriteMode(-999), // unknown mode will cause failure
			},
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			execution: testdata.WFExecRequest(),
			wantErr:   true,
		},
		{
			name: "createOrUpdateCurrentWorkflow step fails",
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteMode(-999), // unknown mode will cause failure
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			execution: testdata.WFExecRequest(),
			wantErr:   true,
		},
		{
			name: "createWorkflowExecutionWithMergeMaps step fails",
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			execution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeAppend), // this will cause failure
			),
			wantErr: true,
		},
		{
			name:                  "executeCreateWorkflowBatchTransaction step fails",
			mapExecuteBatchCASErr: errors.New("some random error"), // this will cause failure
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			execution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeNone),
			),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			session := &fakeSession{
				iter:                      &fakeIter{},
				mapExecuteBatchCASApplied: true,
				mapExecuteBatchCASErr:     tc.mapExecuteBatchCASErr,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			err := db.InsertWorkflowExecutionWithTasks(
				context.Background(),
				tc.workflowRequest,
				tc.request,
				tc.execution,
				tc.transferTasks,
				tc.crossClusterTasks,
				tc.replicationTasks,
				tc.timerTasks,
				tc.shardCondition,
			)

			if (err != nil) != tc.wantErr {
				t.Errorf("InsertWorkflowExecutionWithTasks() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSelectCurrentWorkflow(t *testing.T) {
	tests := []struct {
		name             string
		shardID          int
		domainID         string
		workflowID       string
		currentRunID     string
		lastWriteVersion int64
		createReqID      string
		wantErr          bool
		wantRow          *nosqlplugin.CurrentWorkflowRow
	}{
		{
			name:         "success",
			shardID:      1,
			domainID:     "test-domain-id",
			workflowID:   "test-workflow-id",
			currentRunID: "test-run-id",
			wantErr:      false,
			wantRow: &nosqlplugin.CurrentWorkflowRow{
				ShardID:          1,
				DomainID:         "test-domain-id",
				WorkflowID:       "test-workflow-id",
				RunID:            "test-run-id",
				LastWriteVersion: -24,
			},
		},
		{
			name:         "mapscan failure",
			shardID:      1,
			domainID:     "test-domain-id",
			workflowID:   "test-workflow-id",
			currentRunID: "test-run-id",
			wantErr:      true,
		},
		{
			name:             "lastwriteversion populated",
			shardID:          1,
			domainID:         "test-domain-id",
			workflowID:       "test-workflow-id",
			currentRunID:     "test-run-id",
			lastWriteVersion: 123,
			wantErr:          false,
			wantRow: &nosqlplugin.CurrentWorkflowRow{
				ShardID:          1,
				DomainID:         "test-domain-id",
				WorkflowID:       "test-workflow-id",
				RunID:            "test-run-id",
				LastWriteVersion: 123,
			},
		},
		{
			name:         "create request id populated",
			shardID:      1,
			domainID:     "test-domain-id",
			workflowID:   "test-workflow-id",
			currentRunID: "test-run-id",
			createReqID:  "test-create-request-id",
			wantErr:      false,
			wantRow: &nosqlplugin.CurrentWorkflowRow{
				ShardID:          1,
				DomainID:         "test-domain-id",
				WorkflowID:       "test-workflow-id",
				RunID:            "test-run-id",
				LastWriteVersion: -24,
				CreateRequestID:  "test-create-request-id",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
				mockCurrentRunID := gocql.NewMockUUID(ctrl)
				m["current_run_id"] = mockCurrentRunID
				if !tc.wantErr {
					mockCurrentRunID.EXPECT().String().Return(tc.currentRunID).Times(1)
				}

				execMap := map[string]interface{}{}
				if tc.createReqID != "" {
					mockReqID := gocql.NewMockUUID(ctrl)
					mockReqID.EXPECT().String().Return(tc.createReqID).Times(1)
					execMap["create_request_id"] = mockReqID
				}
				m["execution"] = execMap

				if tc.lastWriteVersion != 0 {
					m["workflow_last_write_version"] = tc.lastWriteVersion
				}

				if tc.wantErr {
					return errors.New("some random error")
				}

				return nil
			}).Times(1)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			row, err := db.SelectCurrentWorkflow(context.Background(), tc.shardID, tc.domainID, tc.workflowID)

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectCurrentWorkflow() error: %v, wantErr?: %v", err, tc.wantErr)
			}

			if err != nil {
				return
			}

			if diff := cmp.Diff(tc.wantRow, row); diff != "" {
				t.Fatalf("Row mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUpdateWorkflowExecutionWithTasks(t *testing.T) {
	tests := []struct {
		name                  string
		workflowRequest       *nosqlplugin.WorkflowRequestsWriteRequest
		request               *nosqlplugin.CurrentWorkflowWriteRequest
		mutatedExecution      *nosqlplugin.WorkflowExecutionRequest
		insertedExecution     *nosqlplugin.WorkflowExecutionRequest
		resetExecution        *nosqlplugin.WorkflowExecutionRequest
		transferTasks         []*nosqlplugin.TransferTask
		crossClusterTasks     []*nosqlplugin.CrossClusterTask
		replicationTasks      []*nosqlplugin.ReplicationTask
		timerTasks            []*nosqlplugin.TimerTask
		shardCondition        *nosqlplugin.ShardCondition
		mapExecuteBatchCASErr error
		wantErr               bool
	}{
		{
			name: "both mutatedExecution and resetExecution not provided",
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			wantErr: true,
		},
		{
			name: "insertOrUpsertWorkflowRequestRow step fails",
			workflowRequest: &nosqlplugin.WorkflowRequestsWriteRequest{
				Rows: []*nosqlplugin.WorkflowRequestRow{
					{
						RequestType: persistence.WorkflowRequestTypeStart,
					},
				},
				WriteMode: nosqlplugin.WorkflowRequestWriteMode(-999), // unknown mode will cause failure
			},
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			mutatedExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeUpdate),
			),
			wantErr: true,
		},
		{
			name: "mutatedExecution provided - success",
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			mutatedExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeUpdate),
			),
		},
		{
			name:    "mutatedExecution provided - update fails",
			wantErr: true,
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			mutatedExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeCreate), // this will cause failure
			),
		},
		{
			name: "resetExecution provided - success",
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			resetExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeClear),
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeReset),
			),
		},
		{
			name:    "resetExecution provided - reset fails",
			wantErr: true,
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			resetExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeNone), // this will cause failure
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeReset),
			),
		},
		{
			name: "resetExecution and insertedExecution provided - success",
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			resetExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeClear),
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeReset),
			),
			insertedExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeNone),
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeCreate),
			),
		},
		{
			name:    "resetExecution and insertedExecution provided - insert fails",
			wantErr: true,
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeNoop,
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			resetExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeClear),
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeReset),
			),
			insertedExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeClear), // this will cause failure
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeCreate),
			),
		},
		{
			name:    "createOrUpdateCurrentWorkflow step fails",
			wantErr: true,
			request: &nosqlplugin.CurrentWorkflowWriteRequest{
				WriteMode: nosqlplugin.CurrentWorkflowWriteModeUpdate,
				Condition: nil, // this will cause failure because Condition must be non-nil for update mode
			},
			shardCondition: &nosqlplugin.ShardCondition{
				ShardID: 1,
			},
			resetExecution: testdata.WFExecRequest(
				testdata.WFExecRequestWithEventBufferWriteMode(nosqlplugin.EventBufferWriteModeClear),
				testdata.WFExecRequestWithMapsWriteMode(nosqlplugin.WorkflowExecutionMapsWriteModeReset),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			session := &fakeSession{
				iter:                      &fakeIter{},
				mapExecuteBatchCASApplied: true,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			err := db.UpdateWorkflowExecutionWithTasks(
				context.Background(),
				tc.workflowRequest,
				tc.request,
				tc.mutatedExecution,
				tc.insertedExecution,
				tc.resetExecution,
				tc.transferTasks,
				tc.crossClusterTasks,
				tc.replicationTasks,
				tc.timerTasks,
				tc.shardCondition,
			)

			if (err != nil) != tc.wantErr {
				t.Errorf("UpdateWorkflowExecutionWithTasks() error: %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestSelectWorkflowExecution(t *testing.T) {
	tests := []struct {
		name        string
		shardID     int
		domainID    string
		workflowID  string
		runID       string
		queryMockFn func(query *gocql.MockQuery)
		wantResp    *nosqlplugin.WorkflowExecution
		wantErr     bool
	}{
		{
			name:       "mapscan failure",
			shardID:    1,
			domainID:   "test-domain-id",
			workflowID: "test-workflow-id",
			runID:      "test-run-id",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					return errors.New("some random error")
				}).Times(1)
			},
			wantErr: true,
		},
		{
			name:       "success",
			shardID:    1,
			domainID:   "test-domain-id",
			workflowID: "test-workflow-id",
			runID:      "test-run-id",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					m["execution"] = map[string]interface{}{}
					m["version_histories"] = []byte{}
					m["version_histories_encoding"] = "thriftrw"
					m["replication_state"] = map[string]interface{}{}
					m["activity_map"] = map[int64]map[string]interface{}{
						1: {"schedule_id": int64(1)},
					}
					m["timer_map"] = map[string]map[string]interface{}{
						"t1": {"started_id": int64(5)},
					}
					m["child_executions_map"] = map[int64]map[string]interface{}{
						3: {"initiated_id": int64(2)},
					}
					m["request_cancel_map"] = map[int64]map[string]interface{}{
						6: {"initiated_id": int64(5)},
					}
					m["signal_map"] = map[int64]map[string]interface{}{
						8: {"initiated_id": int64(7)},
					}
					m["signal_requested"] = []interface{}{
						&fakeUUID{uuid: "aae7b881-48ea-4b23-8d11-aabfd1c1291e"},
					}
					m["buffered_events_list"] = []map[string]interface{}{
						{"encoding_type": "thriftrw", "data": []byte("test-buffered-events-1")},
					}
					m["checksum"] = map[string]interface{}{}
					return nil
				}).Times(1)
			},
			wantResp: &nosqlplugin.WorkflowExecution{
				ExecutionInfo: &persistence.InternalWorkflowExecutionInfo{},
				ActivityInfos: map[int64]*persistence.InternalActivityInfo{
					1: {ScheduleID: 1, DomainID: "test-domain-id"},
				},
				TimerInfos: map[string]*persistence.TimerInfo{
					"t1": {StartedID: 5},
				},
				ChildExecutionInfos: map[int64]*persistence.InternalChildExecutionInfo{
					3: {InitiatedID: 2},
				},
				RequestCancelInfos: map[int64]*persistence.RequestCancelInfo{
					6: {InitiatedID: 5},
				},
				SignalInfos: map[int64]*persistence.SignalInfo{
					8: {InitiatedID: 7},
				},
				SignalRequestedIDs: map[string]struct{}{
					"aae7b881-48ea-4b23-8d11-aabfd1c1291e": {},
				},
				BufferedEvents: []*persistence.DataBlob{
					{Encoding: common.EncodingTypeThriftRW, Data: []byte("test-buffered-events-1")},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)
			session := &fakeSession{
				iter:                      &fakeIter{},
				mapExecuteBatchCASApplied: true,
				query:                     query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}

			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			got, err := db.SelectWorkflowExecution(
				context.Background(),
				tc.shardID,
				tc.domainID,
				tc.workflowID,
				tc.runID,
			)

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectWorkflowExecution() error: %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if diff := cmp.Diff(tc.wantResp, got); diff != "" {
				t.Fatalf("Mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeleteCurrentWorkflow(t *testing.T) {
	tests := []struct {
		name                  string
		shardID               int
		domainID              string
		workflowID            string
		currentRunIDCondition string
		queryMockFn           func(query *gocql.MockQuery)
		wantErr               bool
	}{
		{
			name:                  "success",
			shardID:               1,
			domainID:              "test-domain-id",
			workflowID:            "test-workflow-id",
			currentRunIDCondition: "test-run-id",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:                  "query exec fails",
			shardID:               1,
			domainID:              "test-domain-id",
			workflowID:            "test-workflow-id",
			currentRunIDCondition: "test-run-id",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.DeleteCurrentWorkflow(context.Background(), tc.shardID, tc.domainID, tc.workflowID, tc.currentRunIDCondition)

			if (err != nil) != tc.wantErr {
				t.Errorf("DeleteCurrentWorkflow() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestDeleteWorkflowExecution(t *testing.T) {
	tests := []struct {
		name        string
		shardID     int
		domainID    string
		workflowID  string
		runID       string
		queryMockFn func(query *gocql.MockQuery)
		wantErr     bool
	}{
		{
			name:       "success",
			shardID:    1,
			domainID:   "test-domain-id",
			workflowID: "test-workflow-id",
			runID:      "test-run-id",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:       "query exec fails",
			shardID:    1,
			domainID:   "test-domain-id",
			workflowID: "test-workflow-id",
			runID:      "test-run-id",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.DeleteWorkflowExecution(context.Background(), tc.shardID, tc.domainID, tc.workflowID, tc.runID)

			if (err != nil) != tc.wantErr {
				t.Errorf("DeleteWorkflowExecution() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestSelectAllCurrentWorkflows(t *testing.T) {
	tests := []struct {
		name           string
		shardID        int
		pageToken      []byte
		pageSize       int
		iter           *fakeIter
		wantExecutions []*persistence.CurrentWorkflowExecution
		wantErr        bool
	}{
		{
			name:      "nil iter returned",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			wantErr:   true,
		},
		{
			name:      "run_id is not permanentRunID so excluded from result",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"run_id": &fakeUUID{uuid: "17C305FA-79BB-479E-8AC7-360E956AC01A"},
					},
				},
				pageState: []byte("test-page-token-2"),
			},
			wantExecutions: nil,
		},
		{
			name:      "multiple executions",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"run_id":         &fakeUUID{uuid: permanentRunID},
						"domain_id":      &fakeUUID{uuid: "domain1"},
						"current_run_id": &fakeUUID{uuid: "runid1"},
						"workflow_id":    "wfid1",
						"workflow_state": 1,
					},
					{
						"run_id":         &fakeUUID{uuid: permanentRunID},
						"domain_id":      &fakeUUID{uuid: "domain1"},
						"current_run_id": &fakeUUID{uuid: "runid2"},
						"workflow_id":    "wfid2",
						"workflow_state": 1,
					},
				},
				pageState: []byte("test-page-token-2"),
			},
			wantExecutions: []*persistence.CurrentWorkflowExecution{
				{
					DomainID:     "domain1",
					WorkflowID:   "wfid1",
					RunID:        "30000000-0000-f000-f000-000000000001",
					State:        1,
					CurrentRunID: "runid1",
				},
				{
					DomainID:     "domain1",
					WorkflowID:   "wfid2",
					RunID:        "30000000-0000-f000-f000-000000000001",
					State:        1,
					CurrentRunID: "runid2",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().PageSize(tc.pageSize).Return(query).Times(1)
			query.EXPECT().PageState(tc.pageToken).Return(query).Times(1)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			if tc.iter != nil {
				query.EXPECT().Iter().Return(tc.iter).Times(1)
			} else {
				// Passing tc.iter to Return() doesn't work even though tc.iter is nil due to Go's typed nils.
				// So, we have to call Return(nil) directly.
				query.EXPECT().Iter().Return(nil).Times(1)
			}

			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			gotExecutions, gotPageToken, err := db.SelectAllCurrentWorkflows(context.Background(), tc.shardID, tc.pageToken, tc.pageSize)
			if (err != nil) != tc.wantErr {
				t.Errorf("SelectAllCurrentWorkflows() error: %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if diff := cmp.Diff(tc.wantExecutions, gotExecutions); diff != "" {
				t.Fatalf("Executions mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.iter.pageState, gotPageToken); diff != "" {
				t.Fatalf("Page token mismatch (-want +got):\n%s", diff)
			}

			if !tc.iter.closed {
				t.Error("iter was not closed")
			}
		})
	}
}

func TestSelectAllWorkflowExecutions(t *testing.T) {
	tests := []struct {
		name           string
		shardID        int
		pageToken      []byte
		pageSize       int
		iter           *fakeIter
		wantExecutions []*persistence.InternalListConcreteExecutionsEntity
		wantErr        bool
	}{
		{
			name:      "nil iter returned",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			wantErr:   true,
		},
		{
			name:      "run_id is permanentRunID so excluded from result",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"run_id": &fakeUUID{uuid: "30000000-0000-f000-f000-000000000001"},
					},
				},
				pageState: []byte("test-page-token-2"),
			},
			wantExecutions: nil,
		},
		{
			name:      "multiple executions",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"run_id": &fakeUUID{uuid: "runid1"},
						"execution": map[string]interface{}{
							"domain_id":   &fakeUUID{uuid: "domain1"},
							"workflow_id": "wfid1",
						},
						"version_histories":          []byte("test-version-histories-1"),
						"version_histories_encoding": "thriftrw",
					},
					{
						"run_id": &fakeUUID{uuid: "runid2"},
						"execution": map[string]interface{}{
							"domain_id":   &fakeUUID{uuid: "domain1"},
							"workflow_id": "wfid2",
						},
						"version_histories":          []byte("test-version-histories-1"),
						"version_histories_encoding": "thriftrw",
					},
				},
				pageState: []byte("test-page-token-2"),
			},
			wantExecutions: []*persistence.InternalListConcreteExecutionsEntity{
				{
					ExecutionInfo:    &persistence.InternalWorkflowExecutionInfo{DomainID: "domain1", WorkflowID: "wfid1"},
					VersionHistories: &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("test-version-histories-1")},
				},
				{
					ExecutionInfo:    &persistence.InternalWorkflowExecutionInfo{DomainID: "domain1", WorkflowID: "wfid2"},
					VersionHistories: &persistence.DataBlob{Encoding: "thriftrw", Data: []uint8("test-version-histories-1")},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().PageSize(tc.pageSize).Return(query).Times(1)
			query.EXPECT().PageState(tc.pageToken).Return(query).Times(1)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			if tc.iter != nil {
				query.EXPECT().Iter().Return(tc.iter).Times(1)
			} else {
				// Passing tc.iter to Return() doesn't work even though tc.iter is nil due to Go's typed nils.
				// So, we have to call Return(nil) directly.
				query.EXPECT().Iter().Return(nil).Times(1)
			}

			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			gotExecutions, gotPageToken, err := db.SelectAllWorkflowExecutions(context.Background(), tc.shardID, tc.pageToken, tc.pageSize)
			if (err != nil) != tc.wantErr {
				t.Errorf("SelectAllWorkflowExecutions() error: %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if diff := cmp.Diff(tc.wantExecutions, gotExecutions); diff != "" {
				t.Fatalf("Executions mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.iter.pageState, gotPageToken); diff != "" {
				t.Fatalf("Page token mismatch (-want +got):\n%s", diff)
			}

			if !tc.iter.closed {
				t.Error("iter was not closed")
			}
		})
	}
}

func TestIsWorkflowExecutionExists(t *testing.T) {
	tests := []struct {
		name         string
		queryMockFn  func(query *gocql.MockQuery)
		clientMockFn func(client *gocql.MockClient)
		want         bool
		wantErr      bool
	}{
		{
			name: "success",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).Return(nil).Times(1)
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "not found case returns false but no error",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).Return(errors.New("an error that will be considered as not found err by client mock")).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(true).Times(1)
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "arbitrary error case",
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).Return(errors.New("some error")).Times(1)
			},
			clientMockFn: func(client *gocql.MockClient) {
				client.EXPECT().IsNotFoundError(gomock.Any()).Return(false).Times(1)
			},
			want:    false,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			if tc.clientMockFn != nil {
				tc.clientMockFn(client)
			}

			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			got, err := db.IsWorkflowExecutionExists(context.Background(), 1, "domain1", "wfi", "run1")
			if (err != nil) != tc.wantErr {
				t.Errorf("IsWorkflowExecutionExists() error: %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if got != tc.want {
				t.Errorf("IsWorkflowExecutionExists() got: %v, want: %v", got, tc.want)
			}
		})
	}
}

func TestSelectTransferTasksOrderByTaskID(t *testing.T) {
	tests := []struct {
		name               string
		shardID            int
		pageToken          []byte
		pageSize           int
		exclusiveMinTaskID int64
		inclusiveMaxTaskID int64
		iter               *fakeIter
		wantTasks          []*nosqlplugin.TransferTask
		wantNextPageToken  []byte
		wantErr            bool
	}{
		{
			name:      "nil iter returned",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			wantErr:   true,
		},
		{
			name:      "success",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"transfer": map[string]interface{}{
							"domain_id":   &fakeUUID{uuid: "domain1"},
							"workflow_id": "wfid1",
							"task_id":     int64(1),
						},
					},
					{
						"transfer": map[string]interface{}{
							"domain_id":   &fakeUUID{uuid: "domain2"},
							"workflow_id": "wfid2",
							"task_id":     int64(5),
						},
					},
				},
				pageState: []byte("test-page-token-2"),
			},
			wantTasks: []*persistence.TransferTaskInfo{
				{
					DomainID:   "domain1",
					WorkflowID: "wfid1",
					TaskID:     1,
				},
				{
					DomainID:   "domain2",
					WorkflowID: "wfid2",
					TaskID:     5,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().PageSize(tc.pageSize).Return(query).Times(1)
			query.EXPECT().PageState(tc.pageToken).Return(query).Times(1)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			if tc.iter != nil {
				query.EXPECT().Iter().Return(tc.iter).Times(1)
			} else {
				// Passing tc.iter to Return() doesn't work even though tc.iter is nil due to Go's typed nils.
				// So, we have to call Return(nil) directly.
				query.EXPECT().Iter().Return(nil).Times(1)
			}

			session := &fakeSession{
				query: query,
			}
			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			gotTasks, gotPageToken, err := db.SelectTransferTasksOrderByTaskID(context.Background(), tc.shardID, tc.pageSize, tc.pageToken, tc.exclusiveMinTaskID, tc.inclusiveMaxTaskID)
			if (err != nil) != tc.wantErr {
				t.Errorf("SelectAllWorkflowExecutions() error: %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if diff := cmp.Diff(tc.wantTasks, gotTasks); diff != "" {
				t.Fatalf("Executions mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.iter.pageState, gotPageToken); diff != "" {
				t.Fatalf("Page token mismatch (-want +got):\n%s", diff)
			}

			if !tc.iter.closed {
				t.Error("iter was not closed")
			}
		})
	}
}

func TestDeleteTransferTask(t *testing.T) {
	tests := []struct {
		name        string
		shardID     int
		taskID      int64
		queryMockFn func(query *gocql.MockQuery)
		wantErr     bool
	}{
		{
			name:    "success",
			shardID: 1,
			taskID:  123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:    "query exec fails",
			shardID: 1,
			taskID:  123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.DeleteTransferTask(context.Background(), tc.shardID, tc.taskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("DeleteTransferTask() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestRangeDeleteTransferTasks(t *testing.T) {
	tests := []struct {
		name                 string
		shardID              int
		exclusiveBeginTaskID int64
		inclusiveEndTaskID   int64
		queryMockFn          func(query *gocql.MockQuery)
		wantErr              bool
	}{
		{
			name:                 "success",
			shardID:              1,
			exclusiveBeginTaskID: 123,
			inclusiveEndTaskID:   456,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:                 "query exec fails",
			shardID:              1,
			exclusiveBeginTaskID: 123,
			inclusiveEndTaskID:   456,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.RangeDeleteTransferTasks(context.Background(), tc.shardID, tc.exclusiveBeginTaskID, tc.inclusiveEndTaskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("RangeDeleteTransferTasks() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestSelectTimerTasksOrderByVisibilityTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name              string
		shardID           int
		pageToken         []byte
		pageSize          int
		exclusiveMinTime  time.Time
		inclusiveMaxTime  time.Time
		iter              *fakeIter
		wantTasks         []*nosqlplugin.TimerTask
		wantNextPageToken []byte
		wantErr           bool
	}{
		{
			name:      "nil iter returned",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			wantErr:   true,
		},
		{
			name:      "success",
			shardID:   1,
			pageToken: []byte("test-page-token"),
			pageSize:  10,
			iter: &fakeIter{
				mapScanInputs: []map[string]interface{}{
					{
						"timer": map[string]interface{}{
							"domain_id":     &fakeUUID{uuid: "domain1"},
							"workflow_id":   "wfid1",
							"visibility_ts": now,
						},
					},
					{
						"timer": map[string]interface{}{
							"domain_id":     &fakeUUID{uuid: "domain2"},
							"workflow_id":   "wfid2",
							"visibility_ts": now.Add(time.Hour),
						},
					},
				},
				pageState: []byte("test-page-token-2"),
			},
			wantTasks: []*nosqlplugin.TimerTask{
				{
					DomainID:            "domain1",
					WorkflowID:          "wfid1",
					VisibilityTimestamp: now,
				},
				{
					DomainID:            "domain2",
					WorkflowID:          "wfid2",
					VisibilityTimestamp: now.Add(time.Hour),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			query.EXPECT().PageSize(tc.pageSize).Return(query).Times(1)
			query.EXPECT().PageState(tc.pageToken).Return(query).Times(1)
			query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
			if tc.iter != nil {
				query.EXPECT().Iter().Return(tc.iter).Times(1)
			} else {
				// Passing tc.iter to Return() doesn't work even though tc.iter is nil due to Go's typed nils.
				// So, we have to call Return(nil) directly.
				query.EXPECT().Iter().Return(nil).Times(1)
			}

			session := &fakeSession{
				query: query,
			}

			client := gocql.NewMockClient(ctrl)
			cfg := &config.NoSQL{}
			logger := testlogger.New(t)
			dc := &persistence.DynamicConfiguration{}
			db := newCassandraDBFromSession(cfg, session, logger, dc, dbWithClient(client))

			gotTasks, gotPageToken, err := db.SelectTimerTasksOrderByVisibilityTime(context.Background(), tc.shardID, tc.pageSize, tc.pageToken, tc.exclusiveMinTime, tc.inclusiveMaxTime)
			if (err != nil) != tc.wantErr {
				t.Errorf("SelectTimerTasksOrderByVisibilityTime() error: %v, wantErr %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if diff := cmp.Diff(tc.wantTasks, gotTasks); diff != "" {
				t.Fatalf("Tasks mismatch (-want +got):\n%s", diff)
			}

			if diff := cmp.Diff(tc.iter.pageState, gotPageToken); diff != "" {
				t.Fatalf("Page token mismatch (-want +got):\n%s", diff)
			}

			if !tc.iter.closed {
				t.Error("iter was not closed")
			}

		})
	}
}

func TestDeleteTimerTask(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name                string
		shardID             int
		taskID              int64
		visibilityTimestamp time.Time
		queryMockFn         func(query *gocql.MockQuery)
		wantErr             bool
	}{
		{
			name:                "success",
			shardID:             1,
			taskID:              123,
			visibilityTimestamp: now,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:                "query exec fails",
			shardID:             1,
			taskID:              123,
			visibilityTimestamp: now,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.DeleteTimerTask(context.Background(), tc.shardID, tc.taskID, tc.visibilityTimestamp)

			if (err != nil) != tc.wantErr {
				t.Errorf("DeleteTimerTask() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestRangeDeleteTimerTasks(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name             string
		shardID          int
		inclusiveMinTime time.Time
		exclusiveMaxTime time.Time
		queryMockFn      func(query *gocql.MockQuery)
		wantErr          bool
	}{
		{
			name:             "success",
			shardID:          1,
			inclusiveMinTime: now,
			exclusiveMaxTime: now.Add(time.Hour),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:             "query exec fails",
			shardID:          1,
			inclusiveMinTime: now,
			exclusiveMaxTime: now.Add(time.Hour),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.RangeDeleteTimerTasks(context.Background(), tc.shardID, tc.inclusiveMinTime, tc.exclusiveMaxTime)

			if (err != nil) != tc.wantErr {
				t.Errorf("RangeDeleteTimerTasks() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestSelectReplicationTasksOrderByTaskID(t *testing.T) {
	tests := []struct {
		name               string
		shardID            int
		inclusiveMinTaskID int64
		exclusiveMaxTaskID int64
		pageSize           int
		pageToken          []byte
		queryMockFn        func(query *gocql.MockQuery)
		wantTasks          []*nosqlplugin.ReplicationTask
		wantErr            bool
	}{
		{
			name:               "success",
			shardID:            1,
			inclusiveMinTaskID: 100,
			exclusiveMaxTaskID: 200,
			pageSize:           100,
			pageToken:          []byte("test-page-token"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(100).Return(query).Times(1)
				query.EXPECT().PageState([]byte("test-page-token")).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(&fakeIter{
					mapScanInputs: []map[string]interface{}{
						{
							"replication": map[string]interface{}{
								"domain_id":   &fakeUUID{uuid: "domain1"},
								"workflow_id": "wfid1",
								"task_id":     int64(1),
							},
						},
						{
							"replication": map[string]interface{}{
								"domain_id":   &fakeUUID{uuid: "domain1"},
								"workflow_id": "wfid1",
								"task_id":     int64(2),
							},
						},
					},
				}).Times(1)
			},
			wantTasks: []*nosqlplugin.ReplicationTask{
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 1},
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 2},
			},
		},
		{
			name:               "query iter fails",
			shardID:            1,
			inclusiveMinTaskID: 100,
			exclusiveMaxTaskID: 200,
			pageSize:           100,
			pageToken:          []byte("test-page-token"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(100).Return(query).Times(1)
				query.EXPECT().PageState([]byte("test-page-token")).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(nil).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			gotTasks, _, err := db.SelectReplicationTasksOrderByTaskID(context.Background(), tc.shardID, tc.pageSize, tc.pageToken, tc.inclusiveMinTaskID, tc.exclusiveMaxTaskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectReplicationTasksOrderByTaskID() error: %v, wantErr: %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if diff := cmp.Diff(tc.wantTasks, gotTasks); diff != "" {
				t.Fatalf("Tasks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeleteReplicationTask(t *testing.T) {
	tests := []struct {
		name        string
		shardID     int
		taskID      int64
		queryMockFn func(query *gocql.MockQuery)
		wantErr     bool
	}{
		{
			name:    "success",
			shardID: 1,
			taskID:  123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:    "query exec fails",
			shardID: 1,
			taskID:  123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.DeleteReplicationTask(context.Background(), tc.shardID, tc.taskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("DeleteReplicationTask() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestRangeDeleteReplicationTasks(t *testing.T) {
	tests := []struct {
		name               string
		shardID            int
		inclusiveEndTaskID int64
		queryMockFn        func(query *gocql.MockQuery)
		wantErr            bool
	}{
		{
			name:               "success",
			shardID:            1,
			inclusiveEndTaskID: 123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:               "query exec fails",
			shardID:            1,
			inclusiveEndTaskID: 123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.RangeDeleteReplicationTasks(context.Background(), tc.shardID, tc.inclusiveEndTaskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("RangeDeleteReplicationTasks() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestSelectCrossClusterTasksOrderByTaskID(t *testing.T) {
	tests := []struct {
		name               string
		shardID            int
		inclusiveMinTaskID int64
		exclusiveMaxTaskID int64
		pageSize           int
		pageToken          []byte
		targetCluster      string
		queryMockFn        func(query *gocql.MockQuery)
		wantTasks          []*nosqlplugin.CrossClusterTask
		wantErr            bool
	}{
		{
			name:               "success",
			shardID:            1,
			inclusiveMinTaskID: 100,
			exclusiveMaxTaskID: 200,
			pageSize:           100,
			targetCluster:      "test-target-cluster",
			pageToken:          []byte("test-page-token"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(100).Return(query).Times(1)
				query.EXPECT().PageState([]byte("test-page-token")).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(&fakeIter{
					mapScanInputs: []map[string]interface{}{
						{
							"cross_cluster": map[string]interface{}{
								"domain_id": &fakeUUID{uuid: "domain1"},
								"task_id":   int64(1),
							},
						},
						{
							"cross_cluster": map[string]interface{}{
								"domain_id": &fakeUUID{uuid: "domain2"},
								"task_id":   int64(2),
							},
						},
					},
				}).Times(1)
			},
			wantTasks: []*nosqlplugin.CrossClusterTask{
				{
					TargetCluster: "test-target-cluster",
					TransferTask:  persistence.TransferTaskInfo{DomainID: "domain1", TaskID: 1},
				},
				{
					TargetCluster: "test-target-cluster",
					TransferTask:  persistence.TransferTaskInfo{DomainID: "domain2", TaskID: 2},
				},
			},
		},
		{
			name:               "query iter fails",
			shardID:            1,
			inclusiveMinTaskID: 100,
			exclusiveMaxTaskID: 200,
			pageSize:           100,
			targetCluster:      "test-target-cluster",
			pageToken:          []byte("test-page-token"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(100).Return(query).Times(1)
				query.EXPECT().PageState([]byte("test-page-token")).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(nil).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			gotTasks, _, err := db.SelectCrossClusterTasksOrderByTaskID(context.Background(), tc.shardID, tc.pageSize, tc.pageToken, tc.targetCluster, tc.inclusiveMinTaskID, tc.exclusiveMaxTaskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectCrossClusterTasksOrderByTaskID() error: %v, wantErr: %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if diff := cmp.Diff(tc.wantTasks, gotTasks); diff != "" {
				t.Fatalf("Tasks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDeleteCrossClusterTask(t *testing.T) {
	tests := []struct {
		name          string
		shardID       int
		targetCluster string
		taskID        int64
		queryMockFn   func(query *gocql.MockQuery)
		wantErr       bool
	}{
		{
			name:          "success",
			shardID:       1,
			targetCluster: "test-target-cluster",
			taskID:        123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:          "query exec fails",
			shardID:       1,
			targetCluster: "test-target-cluster",
			taskID:        123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.DeleteCrossClusterTask(context.Background(), tc.shardID, tc.targetCluster, tc.taskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("DeleteCrossClusterTask() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestRangeDeleteCrossClusterTasks(t *testing.T) {
	tests := []struct {
		name                 string
		shardID              int
		targetCluster        string
		exclusiveBeginTaskID int64
		inclusiveEndTaskID   int64
		queryMockFn          func(query *gocql.MockQuery)
		wantErr              bool
	}{
		{
			name:                 "success",
			shardID:              1,
			targetCluster:        "test-target-cluster",
			exclusiveBeginTaskID: 123,
			inclusiveEndTaskID:   456,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:                 "query exec fails",
			shardID:              1,
			targetCluster:        "test-target-cluster",
			exclusiveBeginTaskID: 123,
			inclusiveEndTaskID:   456,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.RangeDeleteCrossClusterTasks(context.Background(), tc.shardID, tc.targetCluster, tc.exclusiveBeginTaskID, tc.inclusiveEndTaskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("RangeDeleteCrossClusterTasks() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestInsertReplicationDLQTask(t *testing.T) {
	tests := []struct {
		name          string
		shardID       int
		sourceCluster string
		taskID        int64
		task          nosqlplugin.ReplicationTask
		queryMockFn   func(query *gocql.MockQuery)
		wantErr       bool
	}{
		{
			name:          "success",
			shardID:       1,
			sourceCluster: "test-source-cluster",
			taskID:        123,
			task: nosqlplugin.ReplicationTask{
				TaskID: 123,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:          "query exec fails",
			shardID:       1,
			sourceCluster: "test-source-cluster",
			taskID:        123,
			task: nosqlplugin.ReplicationTask{
				TaskID: 123,
			},
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.InsertReplicationDLQTask(context.Background(), tc.shardID, tc.sourceCluster, tc.task)

			if (err != nil) != tc.wantErr {
				t.Errorf("RangeDeleteCrossClusterTasks() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestSelectReplicationDLQTasksOrderByTaskID(t *testing.T) {
	tests := []struct {
		name               string
		shardID            int
		inclusiveMinTaskID int64
		exclusiveMaxTaskID int64
		pageSize           int
		pageToken          []byte
		queryMockFn        func(query *gocql.MockQuery)
		wantTasks          []*nosqlplugin.ReplicationTask
		wantErr            bool
	}{
		{
			name:               "success",
			shardID:            1,
			inclusiveMinTaskID: 100,
			exclusiveMaxTaskID: 200,
			pageSize:           100,
			pageToken:          []byte("test-page-token"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(100).Return(query).Times(1)
				query.EXPECT().PageState([]byte("test-page-token")).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(&fakeIter{
					mapScanInputs: []map[string]interface{}{
						{
							"replication": map[string]interface{}{
								"domain_id":   &fakeUUID{uuid: "domain1"},
								"workflow_id": "wfid1",
								"task_id":     int64(1),
							},
						},
						{
							"replication": map[string]interface{}{
								"domain_id":   &fakeUUID{uuid: "domain1"},
								"workflow_id": "wfid1",
								"task_id":     int64(2),
							},
						},
					},
				}).Times(1)
			},
			wantTasks: []*nosqlplugin.ReplicationTask{
				{
					DomainID:   "domain1",
					WorkflowID: "wfid1",
					TaskID:     1,
				},
				{
					DomainID:   "domain1",
					WorkflowID: "wfid1",
					TaskID:     2,
				},
			},
		},
		{
			name:               "query iter fails",
			shardID:            1,
			inclusiveMinTaskID: 100,
			exclusiveMaxTaskID: 200,
			pageSize:           100,
			pageToken:          []byte("test-page-token"),
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().PageSize(100).Return(query).Times(1)
				query.EXPECT().PageState([]byte("test-page-token")).Return(query).Times(1)
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Iter().Return(nil).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			gotTasks, _, err := db.SelectReplicationDLQTasksOrderByTaskID(context.Background(), tc.shardID, "src-cluster", tc.pageSize, tc.pageToken, tc.inclusiveMinTaskID, tc.exclusiveMaxTaskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectReplicationDLQTasksOrderByTaskID() error: %v, wantErr: %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if diff := cmp.Diff(tc.wantTasks, gotTasks); diff != "" {
				t.Fatalf("Tasks mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectReplicationDLQTasksCount(t *testing.T) {
	tests := []struct {
		name        string
		shardID     int
		queryMockFn func(query *gocql.MockQuery)
		wantCount   int64
		wantErr     bool
	}{
		{
			name:    "success",
			shardID: 1,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).DoAndReturn(func(m map[string]interface{}) error {
					m["count"] = int64(42)
					return nil
				}).Times(1)
			},
			wantCount: 42,
		},
		{
			name:    "query mapscan fails",
			shardID: 1,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().MapScan(gomock.Any()).Return(errors.New("failed to scan")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			gotCount, err := db.SelectReplicationDLQTasksCount(context.Background(), tc.shardID, "src-cluster")

			if (err != nil) != tc.wantErr {
				t.Errorf("SelectReplicationDLQTasksCount() error: %v, wantErr: %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr {
				return
			}

			if gotCount != tc.wantCount {
				t.Fatalf("got count %v, want %v", gotCount, tc.wantCount)
			}
		})
	}
}

func TestDeleteReplicationDLQTask(t *testing.T) {
	tests := []struct {
		name          string
		shardID       int
		sourceCluster string
		taskID        int64
		queryMockFn   func(query *gocql.MockQuery)
		wantErr       bool
	}{
		{
			name:          "success",
			shardID:       1,
			sourceCluster: "test-source-cluster",
			taskID:        123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:          "query exec fails",
			shardID:       1,
			sourceCluster: "test-source-cluster",
			taskID:        123,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.DeleteReplicationDLQTask(context.Background(), tc.shardID, tc.sourceCluster, tc.taskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("DeleteReplicationDLQTask() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestRangeDeleteReplicationDLQTasks(t *testing.T) {
	tests := []struct {
		name                 string
		shardID              int
		sourceCluster        string
		inclusiveEndTaskID   int64
		exclusiveBeginTaskID int64
		queryMockFn          func(query *gocql.MockQuery)
		wantErr              bool
	}{
		{
			name:                 "success",
			shardID:              1,
			sourceCluster:        "test-source-cluster",
			inclusiveEndTaskID:   300,
			exclusiveBeginTaskID: 200,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(nil).Times(1)
			},
			wantErr: false,
		},
		{
			name:                 "query exec fails",
			shardID:              1,
			sourceCluster:        "test-source-cluster",
			inclusiveEndTaskID:   300,
			exclusiveBeginTaskID: 200,
			queryMockFn: func(query *gocql.MockQuery) {
				query.EXPECT().WithContext(gomock.Any()).Return(query).Times(1)
				query.EXPECT().Exec().Return(errors.New("failed to exec")).Times(1)
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			query := gocql.NewMockQuery(ctrl)
			tc.queryMockFn(query)

			session := &fakeSession{
				query: query,
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.RangeDeleteReplicationDLQTasks(context.Background(), tc.shardID, tc.sourceCluster, tc.exclusiveBeginTaskID, tc.inclusiveEndTaskID)

			if (err != nil) != tc.wantErr {
				t.Errorf("RangeDeleteReplicationDLQTasks() error: %v, wantErr: %v", err, tc.wantErr)
			}
		})
	}
}

func TestInsertReplicationTask(t *testing.T) {
	tests := []struct {
		name                      string
		tasks                     []*nosqlplugin.ReplicationTask
		shardCondition            nosqlplugin.ShardCondition
		mapExecuteBatchCASApplied bool
		mapExecuteBatchCASPrev    map[string]any
		mapExecuteBatchCASErr     error
		wantErr                   bool
		wantPanic                 bool
	}{
		{
			name:    "no tasks",
			wantErr: false,
		},
		{
			name: "mapExecuteBatchCASErr failure",
			tasks: []*nosqlplugin.ReplicationTask{
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 1},
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 2},
			},
			mapExecuteBatchCASErr: errors.New("failed to execute batch"),
			wantErr:               true,
		},
		{
			name: "not applied and row type not found causes panic",
			tasks: []*nosqlplugin.ReplicationTask{
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 1},
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 2},
			},
			mapExecuteBatchCASApplied: false,
			wantPanic:                 true,
		},
		{
			name: "not applied, row type shard condition failure",
			tasks: []*nosqlplugin.ReplicationTask{
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 1},
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 2},
			},
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"type":     rowTypeShard,
				"range_id": int64(5),
			},
			shardCondition: nosqlplugin.ShardCondition{
				ShardID: 1,
				RangeID: 4, // mismatch with prev range_id causes failure
			},
			wantErr: true,
		},
		{
			name: "not applied, unknown shard condition failure",
			tasks: []*nosqlplugin.ReplicationTask{
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 1},
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 2},
			},
			mapExecuteBatchCASApplied: false,
			mapExecuteBatchCASPrev: map[string]any{
				"type": -1, // not a shard type row
			},
			wantErr: true,
		},
		{
			name: "successfully applied",
			tasks: []*nosqlplugin.ReplicationTask{
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 1},
				{DomainID: "domain1", WorkflowID: "wfid1", TaskID: 2},
			},
			mapExecuteBatchCASApplied: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); (r != nil) != tc.wantPanic {
					t.Errorf("got panic: %v, wantPanic: %v", r, tc.wantPanic)
				}
			}()

			ctrl := gomock.NewController(t)

			session := &fakeSession{
				mapExecuteBatchCASApplied: tc.mapExecuteBatchCASApplied,
				mapExecuteBatchCASPrev:    tc.mapExecuteBatchCASPrev,
				mapExecuteBatchCASErr:     tc.mapExecuteBatchCASErr,
				iter:                      &fakeIter{},
			}
			logger := testlogger.New(t)
			db := newCassandraDBFromSession(nil, session, logger, nil, dbWithClient(gocql.NewMockClient(ctrl)))

			err := db.InsertReplicationTask(context.Background(), tc.tasks, tc.shardCondition)

			if (err != nil) != tc.wantErr {
				t.Errorf("InsertReplicationTask() error: %v, wantErr: %v", err, tc.wantErr)
			}

			if err != nil || tc.wantErr || len(tc.tasks) == 0 {
				return
			}

			if len(session.batches) != 1 {
				t.Fatalf("got %v batches, want 1", len(session.batches))
			}

			if len(tc.tasks)+1 != len(session.batches[0].queries) {
				t.Errorf("got %v batches, want %v", len(session.batches), len(tc.tasks))
			}
		})
	}
}
