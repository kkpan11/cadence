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

package persistence

import "time"

// Task is the generic interface for workflow tasks
type Task interface {
	GetType() int
	GetVersion() int64
	SetVersion(version int64)
	GetTaskID() int64
	SetTaskID(id int64)
	GetVisibilityTimestamp() time.Time
	SetVisibilityTimestamp(timestamp time.Time)
}

type (
	// TaskData is common attributes for all tasks.
	TaskData struct {
		Version             int64
		TaskID              int64
		VisibilityTimestamp time.Time
	}

	// ActivityTask identifies a transfer task for activity
	ActivityTask struct {
		TaskData
		DomainID   string
		TaskList   string
		ScheduleID int64
	}

	// DecisionTask identifies a transfer task for decision
	DecisionTask struct {
		TaskData
		DomainID         string
		TaskList         string
		ScheduleID       int64
		RecordVisibility bool
	}

	// RecordWorkflowStartedTask identifites a transfer task for writing visibility open execution record
	RecordWorkflowStartedTask struct {
		TaskData
	}

	// ResetWorkflowTask identifites a transfer task to reset workflow
	ResetWorkflowTask struct {
		TaskData
	}

	// CloseExecutionTask identifies a transfer task for deletion of execution
	CloseExecutionTask struct {
		TaskData
	}

	// DeleteHistoryEventTask identifies a timer task for deletion of history events of completed execution.
	DeleteHistoryEventTask struct {
		TaskData
	}

	// DecisionTimeoutTask identifies a timeout task.
	DecisionTimeoutTask struct {
		TaskData
		EventID         int64
		ScheduleAttempt int64
		TimeoutType     int
	}

	// WorkflowTimeoutTask identifies a timeout task.
	WorkflowTimeoutTask struct {
		TaskData
	}

	// CancelExecutionTask identifies a transfer task for cancel of execution
	CancelExecutionTask struct {
		TaskData
		TargetDomainID          string
		TargetWorkflowID        string
		TargetRunID             string
		TargetChildWorkflowOnly bool
		InitiatedID             int64
	}

	// SignalExecutionTask identifies a transfer task for signal execution
	SignalExecutionTask struct {
		TaskData
		TargetDomainID          string
		TargetWorkflowID        string
		TargetRunID             string
		TargetChildWorkflowOnly bool
		InitiatedID             int64
	}

	// UpsertWorkflowSearchAttributesTask identifies a transfer task for upsert search attributes
	UpsertWorkflowSearchAttributesTask struct {
		TaskData
	}

	// StartChildExecutionTask identifies a transfer task for starting child execution
	StartChildExecutionTask struct {
		TaskData
		TargetDomainID   string
		TargetWorkflowID string
		InitiatedID      int64
	}

	// RecordWorkflowClosedTask identifies a transfer task for writing visibility close execution record
	RecordWorkflowClosedTask struct {
		TaskData
	}

	// RecordChildExecutionCompletedTask identifies a task for recording the competion of a child workflow
	RecordChildExecutionCompletedTask struct {
		TaskData
		TargetDomainID   string
		TargetWorkflowID string
		TargetRunID      string
	}

	// ApplyParentClosePolicyTask identifies a task for applying parent close policy
	ApplyParentClosePolicyTask struct {
		TaskData
		TargetDomainIDs map[string]struct{}
	}

	// CrossClusterStartChildExecutionTask is the cross-cluster version of StartChildExecutionTask
	CrossClusterStartChildExecutionTask struct {
		StartChildExecutionTask

		TargetCluster string
	}

	// CrossClusterCancelExecutionTask is the cross-cluster version of CancelExecutionTask
	CrossClusterCancelExecutionTask struct {
		CancelExecutionTask

		TargetCluster string
	}

	// CrossClusterSignalExecutionTask is the cross-cluster version of SignalExecutionTask
	CrossClusterSignalExecutionTask struct {
		SignalExecutionTask

		TargetCluster string
	}

	// CrossClusterRecordChildExecutionCompletedTask is the cross-cluster version of RecordChildExecutionCompletedTask
	CrossClusterRecordChildExecutionCompletedTask struct {
		RecordChildExecutionCompletedTask

		TargetCluster string
	}

	// CrossClusterApplyParentClosePolicyTask is the cross-cluster version of ApplyParentClosePolicyTask
	CrossClusterApplyParentClosePolicyTask struct {
		ApplyParentClosePolicyTask

		TargetCluster string
	}

	// ActivityTimeoutTask identifies a timeout task.
	ActivityTimeoutTask struct {
		TaskData
		TimeoutType int
		EventID     int64
		Attempt     int64
	}

	// UserTimerTask identifies a timeout task.
	UserTimerTask struct {
		TaskData
		EventID int64
	}

	// ActivityRetryTimerTask to schedule a retry task for activity
	ActivityRetryTimerTask struct {
		TaskData
		EventID int64
		Attempt int32
	}

	// WorkflowBackoffTimerTask to schedule first decision task for retried workflow
	WorkflowBackoffTimerTask struct {
		TaskData
		EventID     int64 // TODO this attribute is not used?
		TimeoutType int   // 0 for retry, 1 for cron.
	}

	// HistoryReplicationTask is the replication task created for shipping history replication events to other clusters
	HistoryReplicationTask struct {
		TaskData
		FirstEventID      int64
		NextEventID       int64
		BranchToken       []byte
		NewRunBranchToken []byte
	}

	// SyncActivityTask is the replication task created for shipping activity info to other clusters
	SyncActivityTask struct {
		TaskData
		ScheduledID int64
	}

	// FailoverMarkerTask is the marker for graceful failover
	FailoverMarkerTask struct {
		TaskData
		DomainID string
	}
)

// assert all task types implements Task interface
var (
	_ Task = (*ActivityTask)(nil)
	_ Task = (*DecisionTask)(nil)
	_ Task = (*RecordWorkflowStartedTask)(nil)
	_ Task = (*ResetWorkflowTask)(nil)
	_ Task = (*CloseExecutionTask)(nil)
	_ Task = (*DeleteHistoryEventTask)(nil)
	_ Task = (*DecisionTimeoutTask)(nil)
	_ Task = (*WorkflowTimeoutTask)(nil)
	_ Task = (*CancelExecutionTask)(nil)
	_ Task = (*SignalExecutionTask)(nil)
	_ Task = (*RecordChildExecutionCompletedTask)(nil)
	_ Task = (*ApplyParentClosePolicyTask)(nil)
	_ Task = (*UpsertWorkflowSearchAttributesTask)(nil)
	_ Task = (*StartChildExecutionTask)(nil)
	_ Task = (*RecordWorkflowClosedTask)(nil)
	_ Task = (*CrossClusterStartChildExecutionTask)(nil)
	_ Task = (*CrossClusterCancelExecutionTask)(nil)
	_ Task = (*CrossClusterSignalExecutionTask)(nil)
	_ Task = (*CrossClusterRecordChildExecutionCompletedTask)(nil)
	_ Task = (*CrossClusterApplyParentClosePolicyTask)(nil)
	_ Task = (*ActivityTimeoutTask)(nil)
	_ Task = (*UserTimerTask)(nil)
	_ Task = (*ActivityRetryTimerTask)(nil)
	_ Task = (*WorkflowBackoffTimerTask)(nil)
	_ Task = (*HistoryReplicationTask)(nil)
	_ Task = (*SyncActivityTask)(nil)
	_ Task = (*FailoverMarkerTask)(nil)
)

// GetVersion returns the version of the task
func (a *TaskData) GetVersion() int64 {
	return a.Version
}

// SetVersion sets the version of the task
func (a *TaskData) SetVersion(version int64) {
	a.Version = version
}

// GetTaskID returns the sequence ID of the task
func (a *TaskData) GetTaskID() int64 {
	return a.TaskID
}

// SetTaskID sets the sequence ID of the task
func (a *TaskData) SetTaskID(id int64) {
	a.TaskID = id
}

// GetVisibilityTimestamp get the visibility timestamp
func (a *TaskData) GetVisibilityTimestamp() time.Time {
	return a.VisibilityTimestamp
}

// SetVisibilityTimestamp set the visibility timestamp
func (a *TaskData) SetVisibilityTimestamp(timestamp time.Time) {
	a.VisibilityTimestamp = timestamp
}

// GetType returns the type of the activity task
func (a *ActivityTask) GetType() int {
	return TransferTaskTypeActivityTask
}

// GetType returns the type of the decision task
func (d *DecisionTask) GetType() int {
	return TransferTaskTypeDecisionTask
}

// GetVersion returns the version of the decision task
func (d *DecisionTask) GetVersion() int64 {
	return d.Version
}

// GetType returns the type of the record workflow started task
func (a *RecordWorkflowStartedTask) GetType() int {
	return TransferTaskTypeRecordWorkflowStarted
}

// GetType returns the type of the ResetWorkflowTask
func (a *ResetWorkflowTask) GetType() int {
	return TransferTaskTypeResetWorkflow
}

// GetType returns the type of the close execution task
func (a *CloseExecutionTask) GetType() int {
	return TransferTaskTypeCloseExecution
}

// GetType returns the type of the delete execution task
func (a *DeleteHistoryEventTask) GetType() int {
	return TaskTypeDeleteHistoryEvent
}

// GetType returns the type of the timer task
func (d *DecisionTimeoutTask) GetType() int {
	return TaskTypeDecisionTimeout
}

// GetType returns the type of the timer task
func (a *ActivityTimeoutTask) GetType() int {
	return TaskTypeActivityTimeout
}

// GetType returns the type of the timer task
func (u *UserTimerTask) GetType() int {
	return TaskTypeUserTimer
}

// GetType returns the type of the retry timer task
func (r *ActivityRetryTimerTask) GetType() int {
	return TaskTypeActivityRetryTimer
}

// GetType returns the type of the retry timer task
func (r *WorkflowBackoffTimerTask) GetType() int {
	return TaskTypeWorkflowBackoffTimer
}

// GetType returns the type of the timeout task.
func (u *WorkflowTimeoutTask) GetType() int {
	return TaskTypeWorkflowTimeout
}

// GetType returns the type of the cancel transfer task
func (u *CancelExecutionTask) GetType() int {
	return TransferTaskTypeCancelExecution
}

// GetType returns the type of the signal transfer task
func (u *SignalExecutionTask) GetType() int {
	return TransferTaskTypeSignalExecution
}

// GetType returns the type of the record child execution completed task
func (u *RecordChildExecutionCompletedTask) GetType() int {
	return TransferTaskTypeRecordChildExecutionCompleted
}

// GetType returns the type of the apply parent close policy task
func (u *ApplyParentClosePolicyTask) GetType() int {
	return TransferTaskTypeApplyParentClosePolicy
}

// GetType returns the type of the upsert search attributes transfer task
func (u *UpsertWorkflowSearchAttributesTask) GetType() int {
	return TransferTaskTypeUpsertWorkflowSearchAttributes
}

// GetType returns the type of the start child transfer task
func (u *StartChildExecutionTask) GetType() int {
	return TransferTaskTypeStartChildExecution
}

// GetType returns the type of the record workflow closed task
func (u *RecordWorkflowClosedTask) GetType() int {
	return TransferTaskTypeRecordWorkflowClosed
}

// GetType returns of type of the cross-cluster start child task
func (c *CrossClusterStartChildExecutionTask) GetType() int {
	return CrossClusterTaskTypeStartChildExecution
}

// GetType returns of type of the cross-cluster cancel task
func (c *CrossClusterCancelExecutionTask) GetType() int {
	return CrossClusterTaskTypeCancelExecution
}

// GetType returns of type of the cross-cluster signal task
func (c *CrossClusterSignalExecutionTask) GetType() int {
	return CrossClusterTaskTypeSignalExecution
}

// GetType returns of type of the cross-cluster record child workflow completion task
func (c *CrossClusterRecordChildExecutionCompletedTask) GetType() int {
	return CrossClusterTaskTypeRecordChildExeuctionCompleted
}

// GetType returns of type of the cross-cluster cancel task
func (c *CrossClusterApplyParentClosePolicyTask) GetType() int {
	return CrossClusterTaskTypeApplyParentClosePolicy
}

// GetType returns the type of the history replication task
func (a *HistoryReplicationTask) GetType() int {
	return ReplicationTaskTypeHistory
}

// GetType returns the type of the history replication task
func (a *SyncActivityTask) GetType() int {
	return ReplicationTaskTypeSyncActivity
}

// GetType returns the type of the history replication task
func (a *FailoverMarkerTask) GetType() int {
	return ReplicationTaskTypeFailoverMarker
}
