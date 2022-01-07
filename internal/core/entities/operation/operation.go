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

package operation

import (
	"fmt"
	"time"
)

type Status int

const (
	// Operation was created but no worker has started it yet.
	StatusPending Status = iota
	// Operation is being executed by a worker.
	StatusInProgress
	// Operation was canceled, usually by an external action or a timeout.
	StatusCanceled
	// Operation finished with success.
	StatusFinished
	// Operation could not execute because it didn't meet the requirements.
	StatusEvicted
	// Operation finished with error.
	StatusError
)

type Operation struct {
	ID             string
	Status         Status
	DefinitionName string
	SchedulerName  string
	Lease          OperationLease
	CreatedAt      time.Time
}

func (o *Operation) SetLease(lease OperationLease) {
	o.Lease = lease
}

func (s Status) String() (string, error) {
	switch s {
	case StatusPending:
		return "pending", nil
	case StatusInProgress:
		return "in_progress", nil
	case StatusFinished:
		return "finished", nil
	case StatusEvicted:
		return "evicted", nil
	case StatusError:
		return "error", nil
	case StatusCanceled:
		return "canceled", nil
	}

	return "", fmt.Errorf("status could not be mapped to string: %d", s)
}
