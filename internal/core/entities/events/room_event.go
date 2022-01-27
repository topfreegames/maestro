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
	Game      string
	RoomId    string
	Host      string
	Port      int32
	EventType RoomEventType
	PingType  *RoomPingEventType
	Other     map[string]interface{}
}

type RoomEventType string

const (
	Ping      RoomEventType = "resync"
	Arbitrary RoomEventType = "roomEvent"
)

type RoomPingEventType string

const (
	RoomPingReady       RoomPingEventType = "roomReady"
	RoomPingOccupied    RoomPingEventType = "roomOccupied"
	RoomPingTerminating RoomPingEventType = "roomTerminating"
	RoomPingTerminated  RoomPingEventType = "roomTerminated"
)

func ConvertToRoomEventType(value string) (RoomEventType, error) {
	switch value {
	case "resync":
		return Ping, nil
	case "roomEvent":
		return Arbitrary, nil
	default:
		return "", fmt.Errorf("invalid RoomEventType. Should be \"resync\" or \"roomEvent\"")
	}
}

func ConvertToRoomPingEventType(value string) (RoomPingEventType, error) {
	switch value {
	case "roomReady":
		return RoomPingReady, nil
	case "roomOccupied":
		return RoomPingOccupied, nil
	case "roomTerminating":
		return RoomPingTerminating, nil
	case "roomTerminated":
		return RoomPingTerminated, nil
	default:
		return "", fmt.Errorf("invalid RoomPingEventType. Should be \"roomReady\", \"roomOccupied\", \"roomTerminating\" or \"roomTerminated\"")
	}
}
