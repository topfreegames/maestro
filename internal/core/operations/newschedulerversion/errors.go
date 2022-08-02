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

package newschedulerversion

import (
	"fmt"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

type ValidationTimeoutError struct {
	Err      error
	GameRoom *game_room.GameRoom
}

func NewValidationTimeoutError(GameRoom *game_room.GameRoom, err error) *ValidationTimeoutError {
	return &ValidationTimeoutError{Err: err, GameRoom: GameRoom}
}

func (e *ValidationTimeoutError) Unwrap() error {
	return e.Err
}

func (e *ValidationTimeoutError) Is(other error) bool {
	if _, ok := other.(*ValidationTimeoutError); ok {
		return true
	}
	return false
}

func (e ValidationTimeoutError) Error() string {
	return fmt.Sprintf("error validating game room with ID %s, got timeout waiting waiting for room to be ready : %s", e.GameRoom.ID, e.Err.Error())
}

type ValidationPodInErrorError struct {
	Err               error
	GameRoomID        string
	StatusDescription string
}

func NewValidationPodInErrorError(gameRoomID, statusDescription string, err error) *ValidationPodInErrorError {
	return &ValidationPodInErrorError{Err: err, GameRoomID: gameRoomID, StatusDescription: statusDescription}
}

func (e *ValidationPodInErrorError) Unwrap() error {
	return e.Err
}

func (e *ValidationPodInErrorError) Is(other error) bool {
	if _, ok := other.(*ValidationPodInErrorError); ok {
		return true
	}
	return false
}

func (e ValidationPodInErrorError) Error() string {
	return fmt.Sprintf("error validating game room with ID %s, instance is entering in error: %s, %s", e.GameRoomID, e.StatusDescription, e.Err)
}
