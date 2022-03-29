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

import "fmt"

type errorKind int

const (
	ErrKindUnexpected errorKind = iota
	ErrKindInvalidGru
	ErrKindReadyPingTimeout
)

var (
	ErrUnexpected  = &operationExecutionError{kind: ErrKindUnexpected}
	ErrInvalidGru  = &operationExecutionError{kind: ErrKindInvalidGru}
	ErrPingTimeout = &operationExecutionError{kind: ErrKindReadyPingTimeout}
)

type ExecutionError interface {
	Kind() errorKind
	Message() string
	FormattedMessage() string
}

type operationExecutionError struct {
	kind             errorKind
	message          string
	formattedMessage string
}

func (e *operationExecutionError) Kind() errorKind {
	return e.kind
}
func (e *operationExecutionError) Message() string {
	return e.message
}
func (e *operationExecutionError) FormattedMessage() string {
	return e.formattedMessage
}

func NewErrUnexpected(format string, args ...interface{}) *operationExecutionError {
	message := fmt.Sprintf(format, args...)
	return &operationExecutionError{
		kind:    ErrKindUnexpected,
		message: message,
		formattedMessage: fmt.Sprintf("Unexpected Error: %s - Contact the Maestro's responsible team for helping "+
			"troubleshoot.", message),
	}
}

func NewErrInvalidGru(format string, args ...interface{}) *operationExecutionError {
	return &operationExecutionError{
		kind:    ErrKindInvalidGru,
		message: fmt.Sprintf(format, args...),
		formattedMessage: `The GRU could not be validated. Maestro could not wait for game room to be ready,
		either the room is not sending the "ready" ping correctly or it is taking too long to respond. Please check if
		the GRU image is stable, or if roomInitializationTimeoutMillis configuration value needs to be increased, 
		and try again.`,
	}
}

func NewErrReadyPingTimeout(format string, args ...interface{}) *operationExecutionError {
	return &operationExecutionError{
		kind:    ErrKindReadyPingTimeout,
		message: fmt.Sprintf(format, args...),
		formattedMessage: `Got timeout while waiting room status to be ready. Please check if 
		roomInitializationTimeoutMillis configuration value needs to be increased.`,
	}
}
