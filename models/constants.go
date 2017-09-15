// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

//SegmentPostgres represents a segment
const SegmentPostgres = "PostgreSQL"

//SegmentSerialization represents a segment
const SegmentSerialization = "Serialization"

//SegmentModel represents a segment
const SegmentModel = "Model"

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

//SegmentController represents a segment
const SegmentController = "Controller"

//SegmentYaml represents a segment
const SegmentYaml = "Yaml"

//SegmentKubernetes represents a segment
const SegmentKubernetes = "Kubernetes"

//SegmentNamespace represents a segment
const SegmentNamespace = "Kubernetes/Namespace"

//SegmentService represents a segment
const SegmentService = "Kubernetes/Service"

//SegmentPod represents a segment
const SegmentPod = "Kubernetes/Pod"

//SegmentHGetAll represents a segment
const SegmentHGetAll = "Redis/HGetAll"

//SegmentHMSet represents a segment
const SegmentHMSet = "Redis/HMSet"

//SegmentZRangeBy represents a segment
const SegmentZRangeBy = "Redis/ZRangeBy"

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

//StatusTerminating represents a room status
const StatusTerminating = "terminating"

//StatusTerminated represents a room status
const StatusTerminated = "terminated"

//Reason{...} represents reasons for a pod/gru being deleted
const ReasonScaleDown = "scale_down"
const ReasonPingTimeout = "ping_timeout"
const ReasonOccupiedTimeout = "occupied_timeout"
const ReasonUpdate = "update"
const ReasonUpdateError = "update_error"
