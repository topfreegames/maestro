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
