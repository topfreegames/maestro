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
	errUnexpected errorKind = iota
	errAlreadyExists
	errNotFound
	errEncoding
	errInvalidArgument
)

var (
	ErrUnexpected      = &portsError{kind: errUnexpected}
	ErrAlreadyExists   = &portsError{kind: errAlreadyExists}
	ErrNotFound        = &portsError{kind: errNotFound}
	ErrEncoding        = &portsError{kind: errEncoding}
	ErrInvalidArgument = &portsError{kind: errInvalidArgument}
)

type portsError struct {
	kind    errorKind
	message string
	err     error
}

func (e *portsError) Unwrap() error {
	return e.err
}

func (e *portsError) Error() string {
	if e.err == nil {
		return e.message
	}

	return fmt.Sprintf("%s: %s", e.message, e.err)
}

func (e *portsError) Is(other error) bool {
	if err, ok := other.(*portsError); ok {
		return e.kind == err.kind
	}

	return false
}

func (e *portsError) WithError(err error) *portsError {
	e.err = err
	return e
}

func NewErrUnexpected(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errUnexpected,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrAlreadyExists(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errAlreadyExists,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrNotFound(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errNotFound,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrEncoding(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errEncoding,
		message: fmt.Sprintf(format, args...),
	}
}

func NewErrInvalidArgument(format string, args ...interface{}) *portsError {
	return &portsError{
		kind:    errInvalidArgument,
		message: fmt.Sprintf(format, args...),
	}
}
