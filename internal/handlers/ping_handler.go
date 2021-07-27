package handlers

import (
	"context"

	"github.com/topfreegames/maestro/protogen/api"
)

type PingHandler struct{}

func ProvidePingHandler() *PingHandler {
	return &PingHandler{}
}

func (h *PingHandler) GetPing(ctx context.Context, message *api.GetPingMessage) (*api.GetPingMessage, error) {

	return &api.GetPingMessage{
		Message: "pong",
	}, nil

}
