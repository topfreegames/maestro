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

package noop_forwarder

import (
	"context"

	"github.com/topfreegames/maestro/internal/core/entities/events"
	"github.com/topfreegames/maestro/internal/core/entities/forwarder"

	"github.com/topfreegames/maestro/internal/core/entities/game_room"
)

type noopForwarder struct{}

func NewNoopForwarder() *noopForwarder {
	return &noopForwarder{}
}

// ForwardRoomEvent forwards room events. It receives the game room, its instance, and additional attributes.
func (f *noopForwarder) ForwardRoomEvent(ctx context.Context, eventAttributes events.RoomEventAttributes, forwarderOptions forwarder.ForwardOptions) error {
	panic("implement me")
}

// ForwardPlayerEvent forwards a player events. It receives the game room and additional attributes.
func (f *noopForwarder) ForwardPlayerEvent(ctx context.Context, eventAttributes events.PlayerEventAttributes, forwarderOptions forwarder.ForwardOptions) error {
	panic("implement me")
}

// ForwardRoomEventObsolete forwards room events. It receives the game room, its instance, and additional attributes.
func (*noopForwarder) ForwardRoomEventObsolete(ctx context.Context, gameRoom *game_room.GameRoom, instance *game_room.Instance, attributes map[string]interface{}, options interface{}) error {
	return nil
}

// ForwardPlayerEventObsolete forwards a player events. It receives the game room and additional attributes.
func (*noopForwarder) ForwardPlayerEventObsolete(ctx context.Context, gameRoom *game_room.GameRoom, attributes map[string]interface{}, options interface{}) error {
	return nil
}

// Name returns the forwarder name. This name should be unique among other events forwarders.
func (*noopForwarder) Name() string {
	return "noop_forwarder"
}
