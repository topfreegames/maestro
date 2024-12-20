package app

import (
	"context"
	"errors"
	"net/http"

	ginadapter "git.topfreegames.com/maestro/test-service/internal/adapters/gin"
	"git.topfreegames.com/maestro/test-service/internal/core"
	"github.com/gin-gonic/gin"
)

type Server struct {
	RoomManager *core.RoomManager
	httpServer  *http.Server
}

func (s *Server) Start() {
	handler := ginadapter.RoomHandler{RoomManager: s.RoomManager}

	r := gin.Default()
	r.PUT("/room", handler.UpdateGameRoom)
	r.PUT("/status", handler.UpdateStatus)
	r.PUT("/matches", handler.UpdateRunningMatches)

	s.httpServer = &http.Server{
		Addr:    ":8080",
		Handler: r.Handler(),
	}

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
