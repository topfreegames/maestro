// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package eventforwarder

import (
  "context"
  "fmt"
)

func NewRoomStatus(game, roomId, roomType string) *RoomStatus {
	return &RoomStatus{
		Game:   game,
		RoomId: roomId,
		Type:   roomType,
	}
}

//Forward send room or player status to specified server
func (roomStatus *RoomStatus) Forward() error {
	conn, err := grpc.Dial("127.0.0.1:10000")
	if err != nil {
		return err
	}
	defer conn.Close()

  client := NewSenderClient(conn)
  response, err := client.SendRoomStatus(context.Backgound(), roomStatus)
  fmt.Println(response.Message)

	return nil
}

func main() {
  roomStatus := NewRoomStatus("henrod", "henrod", "henrod")
  err := roomStatus.Forward()
  fmt.Println(err)
}
