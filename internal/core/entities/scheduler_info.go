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

package entities

import "github.com/topfreegames/maestro/internal/core/entities/autoscaling"

type AutoscalingInfo struct {
	Enabled bool
	Min     int
	Max     int
}

type SchedulerInfo struct {
	Name             string
	Game             string
	State            string
	RoomsReplicas    int
	RoomsReady       int
	RoomsOccupied    int
	RoomsPending     int
	RoomsTerminating int
	Autoscaling      *AutoscalingInfo
}

func NewSchedulerInfo(opts ...SchedulerInfoOption) (schedulerInfo *SchedulerInfo) {
	schedulerInfo = &SchedulerInfo{}
	for _, opt := range opts {
		opt(schedulerInfo)
	}

	return
}

type SchedulerInfoOption func(*SchedulerInfo)

func WithName(name string) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		schedulerInfo.Name = name
	}
}
func WithGame(game string) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		schedulerInfo.Game = game
	}
}

func WithState(state string) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		schedulerInfo.State = state
	}
}

func WithRoomsReplicas(roomsReplicas int) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		schedulerInfo.RoomsReplicas = roomsReplicas
	}
}

func WithRoomsReady(roomsReady int) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		schedulerInfo.RoomsReady = roomsReady
	}
}
func WithRoomsOccupied(roomsOccupied int) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		schedulerInfo.RoomsOccupied = roomsOccupied
	}
}
func WithRoomsPending(roomsPending int) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		schedulerInfo.RoomsPending = roomsPending
	}
}
func WithRoomsTerminating(roomsTerminating int) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		schedulerInfo.RoomsTerminating = roomsTerminating
	}
}

func WithAutoscalingInfo(schedulerAutoscaling *autoscaling.Autoscaling) SchedulerInfoOption {
	return func(schedulerInfo *SchedulerInfo) {
		if schedulerAutoscaling != nil {
			autoscalingInfo := &AutoscalingInfo{
				Enabled: schedulerAutoscaling.Enabled,
				Min:     schedulerAutoscaling.Min,
				Max:     schedulerAutoscaling.Max,
			}
			schedulerInfo.Autoscaling = autoscalingInfo
		}
	}
}
