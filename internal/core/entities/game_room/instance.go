// MIT License
//
// Copyright (c) 2021 TFG Co
//
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

package game_room

import "fmt"

// InstanceStatusType represents the status that a game room instance has.
type InstanceStatusType int

const (
	// Instance has an undefined status.
	InstanceUnknown InstanceStatusType = iota
	// Instance is not ready yet. It will be on this state while creating.
	InstancePending
	// Instance is running fine and all probes are being successful.
	InstanceReady
	// Instance has received a termination signal and it is on process of
	// shutdown.
	InstanceTerminating
	// Instance has some sort of error.
	InstanceError
)

func (statusType InstanceStatusType) String() string {
	switch statusType {
	case InstanceUnknown:
		return "unknown"
	case InstancePending:
		return "pending"
	case InstanceReady:
		return "ready"
	case InstanceTerminating:
		return "terminating"
	case InstanceError:
		return "error"
	default:
		panic(fmt.Sprintf("invalid value for InstanceStatusType : %d", int(statusType)))
	}
}

type InstanceStatus struct {
	Type InstanceStatusType `json:"type"`
	// Description has more information about the status. For example, if we have a
	// status Error, it will tell us which error it is.
	Description string `json:"description"`
}

type Port struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

type Address struct {
	Host  string `json:"host"`
	Ports []Port `json:"ports"`
}

type Instance struct {
	ID          string         `json:"id"`
	SchedulerID string         `json:"schedulerId"`
	Version     string         `json:"version"`
	Status      InstanceStatus `json:"status"`
	Address     *Address       `json:"address"`
}
