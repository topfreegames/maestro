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

package gin

import (
	"fmt"

	"git.topfreegames.com/maestro/test-service/internal/core"
	"github.com/gin-gonic/gin"
)

type RoomHandler struct {
	RoomManager *core.RoomManager
}

type Request struct {
	Status         string `form:"status"`
	RunningMatches int    `form:"running_matches"`
}

func (h *RoomHandler) UpdateGameRoom(c *gin.Context) {
	status, runningMatches, err := parseRequest(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	room := h.RoomManager.UpdateGameRoom(status, runningMatches)

	c.JSON(200, gin.H{"status": room.Status, "runningMatches": room.RunningMatches})
}

func (h *RoomHandler) UpdateStatus(c *gin.Context) {
	status, _, err := parseRequest(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	room := h.RoomManager.UpdateStatus(status)

	c.JSON(200, gin.H{"status": room.Status, "runningMatches": room.RunningMatches})
}

func (h *RoomHandler) UpdateRunningMatches(c *gin.Context) {
	_, runningMatches, err := parseRequest(c)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	room := h.RoomManager.UpdateRunningMatches(runningMatches)

	c.JSON(200, gin.H{"status": room.Status, "runningMatches": room.RunningMatches})
}

func parseRequest(c *gin.Context) (core.Status, int, error) {
	var req Request

	if c.ShouldBind(&req) != nil {
		return core.Unknown, 0, fmt.Errorf("invalid request")
	}

	status, err := core.StatusFromString(req.Status)
	if err != nil {
		return core.Unknown, 0, err
	}

	return status, req.RunningMatches, nil
}
