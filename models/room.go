// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"github.com/topfreegames/extensions/interfaces"
)

// Room is the struct that defines a room in maestro
type Room struct {
	ID       string `db:"id"`
	ConfigID string `db:"config_id"`
	Status   string `db:"status"`
}

// RoomsStatus is the struct that defines a rooms status
type RoomsStatus struct {
	Count  int
	Status string
}

// NewRoom is the room constructor
func NewRoom(id, configID string) *Room {
	return &Room{
		ID:       id,
		ConfigID: configID,
		Status:   "creating",
	}
}

// Create creates a room in the database
func (r *Room) Create(db interfaces.DB) error {
	_, err := db.Query(r, `
		INSERT INTO rooms (id, config_id, status) VALUES (?id, ?config_id, ?status)
		RETURNING id
	`, r)
	return err
}

// SetStatus updates the status of a given room in the database
func (r *Room) SetStatus(db interfaces.DB, status string) error {
	_, err := db.Query(r, `UPDATE rooms SET status = ? WHERE id = ?`, status, r.ConfigID)
	return err
}

// GetRoomsCountByStatus returns the count of rooms for each status
func GetRoomsCountByStatus(db interfaces.DB, configID string) (map[string]int, error) {
	roomStatuses := []*RoomsStatus{}
	_, err := db.Query(
		roomStatuses,
		`SELECT COUNT(*) as count, status FROM rooms WHERE config_id = ? GROUP BY status`,
		configID,
	)
	if err != nil {
		return nil, err
	}
	countByStatus := map[string]int{}
	for _, rs := range roomStatuses {
		countByStatus[rs.Status] = rs.Count
	}
	return countByStatus, nil
}
