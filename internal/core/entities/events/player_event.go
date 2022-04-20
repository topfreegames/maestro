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

type PlayerEventAttributes struct {
	RoomId    string
	PlayerId  string
	EventType PlayerEventType
	Other     map[string]interface{}
}

type PlayerEventType string

const (
	PlayerLeft PlayerEventType = "playerLeft"
	PlayerJoin PlayerEventType = "playerJoin"
)

func ConvertToPlayerEventType(value string) (PlayerEventType, error) {
	switch value {
	case "playerLeft":
		return PlayerLeft, nil
	case "playerJoin":
		return PlayerJoin, nil
	default:
		return "", fmt.Errorf("invalid PlayerEventType. Should be \"playerLeft\" or \"playerJoin\"")
	}
}
