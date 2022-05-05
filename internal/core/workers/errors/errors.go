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

package errors

import "fmt"

type errorKind int

const (
	errOperationWithInvalidStatus errorKind = iota
	errGrantLeaseFailed
	errStartOperationFailed
)

var (
	ErrOperationWithInvalidStatus = &serviceError{kind: errOperationWithInvalidStatus}
	ErrGrantLeaseFailed           = &serviceError{kind: errGrantLeaseFailed}
	ErrStartOperationFailed       = &serviceError{kind: errStartOperationFailed}
)

type serviceError struct {
	kind    errorKind
	message string
	err     error
}

func (e *serviceError) Error() string {
	if e.err == nil {
		return e.message
	}
	if e.message == "" {
		return e.err.Error()
	}

	return fmt.Sprintf("%s: %s", e.message, e.err)
}

func (e *serviceError) Is(other error) bool {
	if err, ok := other.(*serviceError); ok {
		return e.kind == err.kind
	}

	return false
}

func (e *serviceError) WithError(err error) *serviceError {
	e.err = err
	return e
}

func NewErrOperationWithInvalidStatus(format string, args ...interface{}) *serviceError {
	return &serviceError{
		kind:    errOperationWithInvalidStatus,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrGrantLeaseFailed(format string, args ...interface{}) *serviceError {
	return &serviceError{
		kind:    errGrantLeaseFailed,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrStartOperationFailed(format string, args ...interface{}) *serviceError {
	return &serviceError{
		kind:    errStartOperationFailed,
		message: fmt.Sprintf(format, args...),
	}
}
