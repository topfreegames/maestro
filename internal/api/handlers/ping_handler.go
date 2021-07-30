package handlers

import (
	"context"

	api "github.com/topfreegames/maestro/pkg/api/v1"
)

type PingHandler struct {
	api.UnimplementedPingServer
}

func ProvidePingHandler() *PingHandler {
	return &PingHandler{}
}

func (PingHandler) GetPing(ctx context.Context, message *api.GetPingMessage) (*api.GetPingMessage, error) {

	return &api.GetPingMessage{
		Message: "pong",
	}, nil

}
