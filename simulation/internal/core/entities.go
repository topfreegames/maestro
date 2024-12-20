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

package core

import "fmt"

type Status int

const (
	Unknown Status = iota - 1
	Pending
	Unready
	Ready
	Occupied
	Terminating
	Error
	Terminated
	Active
)

func (s Status) String() string {
	if s == Unknown {
		return "unknown"
	}

	return []string{"pending", "unready", "ready", "occupied", "terminating", "error", "terminated", "active"}[s]
}

func StatusFromString(value string) (Status, error) {
	statuses := map[string]Status{
		"pending":     Pending,
		"unready":     Unready,
		"ready":       Ready,
		"occupied":    Occupied,
		"terminating": Terminating,
		"error":       Error,
		"terminated":  Terminated,
		"active":      Active,
	}

	if status, ok := statuses[value]; ok {
		return status, nil
	}

	return Unknown, fmt.Errorf("invalid value for Status: %s", value)
}

type Room struct {
	ID             string
	Scheduler      string
	Status         Status
	RunningMatches int
}
