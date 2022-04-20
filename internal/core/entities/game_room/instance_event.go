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

type InstanceEventType int

const (
	// InstanceEventTypeAdded will happen when the game instance is
	// added on the runtime. Happen only once per instance.
	InstanceEventTypeAdded InstanceEventType = iota
	// InstanceEventTypeUpdated will happen when the instance has any
	// change on the runtime, for example, if its status change. Can happen
	// multiple times for an instance.
	InstanceEventTypeUpdated
	// InstanceEventTypeDeleted will happen when the instance is
	// deleted from the runtime. Can happen only once per instance.
	InstanceEventTypeDeleted
)

func (eventType InstanceEventType) String() string {
	switch eventType {
	case InstanceEventTypeAdded:
		return "added"
	case InstanceEventTypeUpdated:
		return "updated"
	case InstanceEventTypeDeleted:
		return "deleted"
	default:
		panic(fmt.Sprintf("invalid value for InstanceEventType: %d", int(eventType)))
	}
}

// InstanceEvent this struct represents an event that happened on the
// run time, check InstanceEventType to see which event is available.
type InstanceEvent struct {
	Type     InstanceEventType
	Instance *Instance
}
