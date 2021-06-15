package game_room

// StatusEvent an event with the new game room status.
type StatusEvent struct {
	RoomID        string
	SchedulerName string
	Status        GameRoomStatus
}
