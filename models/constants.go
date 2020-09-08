// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

//SegmentInsert represents a segment
const SegmentInsert = "Database/Insert"

//SegmentDelete represents a segment
const SegmentDelete = "Database/Delete"

//SegmentUpdate represents a segment
const SegmentUpdate = "Database/Update"

//SegmentUpsert represents a segment
const SegmentUpsert = "Database/Upsert"

//SegmentSelect represents a segment
const SegmentSelect = "Database/Select"

//SegmentGroupBy represents a segment
const SegmentGroupBy = "Database/GroupBy"

//SegmentNamespace represents a segment
const SegmentNamespace = "Kubernetes/Namespace"

//SegmentService represents a segment
const SegmentService = "Kubernetes/Service"

//SegmentPod represents a segment
const SegmentPod = "Kubernetes/Pod"

//SegmentMetrics represents a segment
const SegmentMetrics = "Kubernetes/Metrics"

//SegmentHGetAll represents a segment
const SegmentHGetAll = "Redis/HGetAll"

//SegmentHMSet represents a segment
const SegmentHMSet = "Redis/HMSet"

//SegmentZRangeBy represents a segment
const SegmentZRangeBy = "Redis/ZRangeBy"

//SegmentSMembers represents a segment
const SegmentSMembers = "Redis/SMembers"

//SegmentSAdd represents a segment
const SegmentSAdd = "Redis/SAdd"

//SegmentSRem represents a segment
const SegmentSRem = "Redis/SRem"

//SegmentGet represents a segment
const SegmentGet = "Redis/Get"

//SegmentSIsMember represents a segment
const SegmentSIsMember = "Redis/SIsMember"

//SegmentSRandMember represents a segment
const SegmentSRandMember = "Redis/SegmentSRandMember"

//SegmentPipeExec represents a segment
const SegmentPipeExec = "Redis/Exec"

//SegmentSet represents a segment
const SegmentSet = "Redis/Set"

//StateCreating represents a cluster state
const StateCreating = "creating"

//StateTerminating represents a cluster state
const StateTerminating = "terminating"

//StateInSync represents a cluster state
const StateInSync = "in-sync"

//StateSubdimensioned represents a cluster state
const StateSubdimensioned = "subdimensioned"

//StateOverdimensioned represents a cluster state
const StateOverdimensioned = "overdimensioned"

//StatusCreating represents a room status
const StatusCreating = "creating"

//StatusReady represents a room status
const StatusReady = "ready"

//StatusOccupied represents a room status
const StatusOccupied = "occupied"

//StatusReadyOrOccupied represents an aggregate of room status
const StatusReadyOrOccupied = "ready_or_occupied"

//StatusTerminating represents a room status
const StatusTerminating = "terminating"

//StatusTerminated represents a room status
const StatusTerminated = "terminated"

// Million is an int64 equals to 1M
const Million int64 = 1000 * 1000

// GlobalPortsPoolKey is the key on redis that saves the range of ports used for pods
const GlobalPortsPoolKey = "maestro:free:ports:global:range"

// Global is the string global
const Global = "global"

// PodNotFitsHostPorts is a message when the pod's host port is no available in any node of the pool
const PodNotFitsHostPorts = "PodFitsHostPorts"

// InvalidPodWaitingStates are all the states that are not accepted in a waiting pod.
var InvalidPodWaitingStates = []string{
	"ErrImageNeverPull",
	"ErrImagePullBackOff",
	"ImagePullBackOff",
	"ErrInvalidImageName",
	"ErrImagePull",
	"CrashLoopBackOff",
}

// OperationManager description constants

// OpManagerRunning constant
const OpManagerRunning = "running"

// OpManagerRollingUpdate constant
const OpManagerRollingUpdate = "rolling update"

// OpManagerWaitingLock constant
const OpManagerWaitingLock = "waiting for lock"

// OpManagerFinished constant
const OpManagerFinished = "finished"

// OpManagerErrored constant
const OpManagerErrored = "errored"

// OpManagerTimedout constant
const OpManagerTimedout = "timedout"
