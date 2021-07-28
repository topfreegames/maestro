package handlers

import (
	"context"

	api "github.com/topfreegames/maestro/protogen/api/v1"
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
