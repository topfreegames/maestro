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

package events

import "fmt"

type RoomEventAttributes struct {
	Game           string
	RoomId         string
	Host           string
	Port           int32
	EventType      RoomEventType
	RoomStatusType *RoomStatusType
	Other          map[string]interface{}
}

type RoomEventType string

const (
	Ping      RoomEventType = "resync"
	Arbitrary RoomEventType = "roomEvent"
	Status    RoomEventType = "status"
)

type RoomStatusType string

const (
	RoomStatusReady       RoomStatusType = "roomReady"
	RoomStatusOccupied    RoomStatusType = "roomOccupied"
	RoomStatusTerminating RoomStatusType = "roomTerminating"
	RoomStatusTerminated  RoomStatusType = "roomTerminated"
)

func ConvertToRoomEventType(value string) (RoomEventType, error) {
	switch value {
	case string(Ping):
		return Ping, nil
	case string(Arbitrary):
		return Arbitrary, nil
	case string(Status):
		return Status, nil
	default:
		return "", fmt.Errorf("invalid RoomEventType. Should be \"resync\" or \"roomEvent\"")
	}
}

func FromRoomEventTypeToString(eventType RoomEventType) string {
	switch eventType {
	case Ping:
		return "resync"
	case Arbitrary:
		return "roomEvent"
	case Status:
		return "roomStatus"
	default:
		return ""
	}
}

func ConvertToRoomPingEventType(value string) (RoomStatusType, error) {
	switch value {
	case "ready":
		return RoomStatusReady, nil
	case "occupied":
		return RoomStatusOccupied, nil
	case "terminating":
		return RoomStatusTerminating, nil
	case "terminated":
		return RoomStatusTerminated, nil
	default:
		return "", fmt.Errorf("invalid RoomStatusType. Should be \"ready\", \"occupied\", \"terminating\" or \"terminated\"")
	}
}
