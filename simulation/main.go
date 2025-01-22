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

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.topfreegames.com/maestro/test-service/app"
	"git.topfreegames.com/maestro/test-service/internal/core"
)

func main() {
	config := core.LoadConfig()

	roomStatus, err := core.StatusFromString(config.InitialStatus)
	if err != nil {
		log.Fatalf("error parsing initial status: %v", err)
	}

	room := &core.Room{
		ID:             config.Room.ID,
		Scheduler:      config.Scheduler.Name,
		Status:         roomStatus,
		RunningMatches: 0,
	}

	roomManager := core.NewRoomManager(room)
	worker := core.NewWorker(room, config)

	server := &app.Server{RoomManager: roomManager}
	go server.Start()

	wrk := &app.Worker{Worker: worker}
	go wrk.Start()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server shutdown: ", err)
	}

	wrk.Stop(ctx)

	select {
	case <-ctx.Done():
		log.Println("Server shutdown timed out.")
	}
}
