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
