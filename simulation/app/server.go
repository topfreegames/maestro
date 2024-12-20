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
