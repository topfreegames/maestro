package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
)

type MaestroGameRoomsManager struct {
	roomsMap map[string]GameRoomPingStatus
}

func (mm MaestroGameRoomsManager) registerRoomWithInitialStatus(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, ok := vars["roomID"]
	if !ok {
		Write(resp, http.StatusBadRequest, "roomID is missing in parameters")
		return
	}

	mm.roomsMap[id] = GameRoomPingStatusReady

	Write(resp, http.StatusOK, "room registered with success")
}

func (mm MaestroGameRoomsManager) retrieveTargetStatusToRoom(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, ok := vars["roomID"]
	if !ok {
		Write(resp, http.StatusBadRequest, "roomID is missing in parameters")
		return
	}
	roomStatus, ok := mm.roomsMap[id]
	if !ok {
		Write(resp, http.StatusNotFound, "roomID is not registered")
		return
	}

	Write(resp, http.StatusOK, fmt.Sprintf(`{"status": "%s" }`, roomStatus.String()))
}

func (mm MaestroGameRoomsManager) setRoomStatus(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, ok := vars["roomID"]
	if !ok {
		Write(resp, http.StatusBadRequest, "roomID is missing in parameters")
		return
	}
	status, ok := vars["status"]
	if !ok {
		Write(resp, http.StatusBadRequest, "status is missing in parameters")
		return
	}

	gameRoomPingStatus, err := FromStringToGameRoomPingStatus(status)
	if err != nil {
		Write(resp, http.StatusBadRequest, "status is not valid")
		return
	}

	mm.roomsMap[id] = gameRoomPingStatus

	Write(resp, http.StatusOK, "status for room updated with success")
}

func main() {
	fmt.Printf("Starting Maestro Game Rooms Manager\n")

	manager := MaestroGameRoomsManager{
		roomsMap: make(map[string]GameRoomPingStatus),
	}

	router := mux.NewRouter()

	// Routes for game rooms
	router.HandleFunc("/register/{roomID}", manager.registerRoomWithInitialStatus).Methods("POST")
	router.HandleFunc("/getStatus/{roomID}", manager.retrieveTargetStatusToRoom).Methods("GET")

	// Routes for management
	router.HandleFunc("/setStatus/{roomID}/{status}", manager.setRoomStatus).Methods("POST")

	http.ListenAndServe(":8080", router)
}

// GameRoomPingStatus room status informed through ping.
type GameRoomPingStatus int

const (
	// GameRoomPingStatusUnknown room hasn't sent a ping yet.
	GameRoomPingStatusUnknown GameRoomPingStatus = iota
	// GameRoomPingStatusReady room is empty and ready to receive matches
	GameRoomPingStatusReady
	// GameRoomPingStatusOccupied room has matches running inside of it.
	GameRoomPingStatusOccupied
	// GameRoomPingStatusTerminating room is in the process of terminating itself.
	GameRoomPingStatusTerminating
	// GameRoomPingStatusTerminated room has terminated.
	GameRoomPingStatusTerminated
)

func (status GameRoomPingStatus) String() string {
	switch status {
	case GameRoomPingStatusUnknown:
		return "unknown"
	case GameRoomPingStatusReady:
		return "ready"
	case GameRoomPingStatusOccupied:
		return "occupied"
	case GameRoomPingStatusTerminating:
		return "terminating"
	case GameRoomPingStatusTerminated:
		return "terminated"
	default:
		panic(fmt.Sprintf("invalid value for GameRoomPingStatus: %d", int(status)))
	}
}

func FromStringToGameRoomPingStatus(value string) (GameRoomPingStatus, error) {
	switch value {
	case "ready":
		return GameRoomPingStatusReady, nil
	case "occupied":
		return GameRoomPingStatusOccupied, nil
	case "terminating":
		return GameRoomPingStatusTerminating, nil
	case "terminated":
		return GameRoomPingStatusTerminated, nil
	default:
	}

	return GameRoomPingStatusUnknown, fmt.Errorf("cannot convert string %s to a valid GameRoomStatus representation", value)
}

//Write to the response and with the status code
func Write(w http.ResponseWriter, status int, text string) {
	WriteBytes(w, status, []byte(text))
}

//WriteBytes to the response and with the status code
func WriteBytes(w http.ResponseWriter, status int, text []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(text)
}
