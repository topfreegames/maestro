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

package operations

import (
	"context"
	"fmt"
	"strings"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

type errorKind int

const (
	ErrKindUnexpected errorKind = iota
	ErrKindInvalidGru
	ErrKindReadyPingTimeout
	ErrKindTerminatingPingTimeout
)

var (
	ErrUnexpected             = &operationExecutionError{kind: ErrKindUnexpected}
	ErrInvalidGru             = &operationExecutionError{kind: ErrKindInvalidGru}
	ErrReadyPingTimeout       = &operationExecutionError{kind: ErrKindReadyPingTimeout}
	ErrTerminatingPingTimeout = &operationExecutionError{kind: ErrKindReadyPingTimeout}
)

type ExecutionError interface {
	Kind() errorKind
	FormattedMessage() string
	Error() error
	IsContextCanceled() bool
}

type operationExecutionError struct {
	kind             errorKind
	formattedMessage string
	err              error
}

func (e *operationExecutionError) Error() error {
	return e.err
}

func (e *operationExecutionError) Kind() errorKind {
	return e.kind
}

func (e *operationExecutionError) FormattedMessage() string {
	return e.formattedMessage
}

func (e *operationExecutionError) IsContextCanceled() bool {
	return strings.Contains(e.err.Error(), context.Canceled.Error())
}

func NewErrUnexpected(err error) *operationExecutionError {
	message := err.Error()
	return &operationExecutionError{
		kind: ErrKindUnexpected,
		formattedMessage: fmt.Sprintf("Unexpected Error: %s - Contact the Maestro's responsible team for helping "+
			"troubleshoot.", message),
		err: err,
	}
}

func NewErrInvalidGru(gameRoom *game_room.GameRoom, err error) *operationExecutionError {
	return &operationExecutionError{
		kind: ErrKindInvalidGru,
		formattedMessage: fmt.Sprintf(`The GRU could not be validated. Maestro got timeout waiting for the GRU with ID: %s-%s to be ready. You can check if
		the GRU image is stable on the its logs. If you could not spot any issues, please contact us.`, gameRoom.SchedulerID, gameRoom.ID),
		err: err,
	}
}

func NewErrReadyPingTimeout(err error) *operationExecutionError {
	return &operationExecutionError{
		kind: ErrKindReadyPingTimeout,
		formattedMessage: `Got timeout while waiting room status to be ready. You can check if 
		roomInitializationTimeoutMillis configuration value needs to be increased.`,
		err: err,
	}
}

func NewErrTerminatingPingTimeout(err error) *operationExecutionError {
	return &operationExecutionError{
		kind: ErrKindTerminatingPingTimeout,
		formattedMessage: `Got timeout while waiting room status to be terminating. You can check if 
		roomDeletionTimeoutMillis configuration value needs to be increased.`,
		err: err,
	}
}
