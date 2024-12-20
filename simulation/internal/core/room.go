package core

type RoomManager struct {
	room *Room
}

func NewRoomManager(room *Room) *RoomManager {
	return &RoomManager{room: room}
}

func (m *RoomManager) UpdateGameRoom(status Status, runningMatches int) *Room {
	m.room.Status = status
	m.room.RunningMatches = runningMatches

	return m.room
}

func (m *RoomManager) UpdateStatus(status Status) *Room {
	return m.UpdateGameRoom(status, m.room.RunningMatches)
}

func (m *RoomManager) UpdateRunningMatches(runningMatches int) *Room {
	return m.UpdateGameRoom(m.room.Status, runningMatches)
}
