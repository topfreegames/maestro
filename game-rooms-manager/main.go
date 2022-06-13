package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type MaestroGameRoomsManager struct {
	roomsMap          map[string]GameRoomPingStatus
	roomsCreatedAtMap map[string]time.Time
	autoscalingMode   bool
	mutex             *sync.RWMutex
}

const (
	autoscalingModeRoomTimeout = time.Second*30
)

// Routes for game rooms

func (mm *MaestroGameRoomsManager) registerRoomWithInitialStatus(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, ok := vars["roomID"]
	if !ok {
		Write(resp, http.StatusBadRequest, "roomID is missing in parameters")
		return
	}

	mm.writeMap(id, GameRoomPingStatusReady)
	mm.writeCreatedAtMap(id)

	Write(resp, http.StatusOK, "room registered with success")
}

func (mm *MaestroGameRoomsManager) retrieveTargetStatusToRoom(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, ok := vars["roomID"]
	if !ok {
		Write(resp, http.StatusBadRequest, "roomID is missing in parameters")
		return
	}
	roomStatus, ok := mm.readMap(id)
	if !ok {
		Write(resp, http.StatusNotFound, "roomID is not registered")
		return
	}

	if mm.autoscalingMode {
		createdAt, ok := mm.readCreatedAtMap(id)
		if !ok {
			fmt.Printf("failed to get createdAt info\n")
		} else {
			age := time.Since(createdAt)
			if age > autoscalingModeRoomTimeout {
				Write(resp, http.StatusOK, `{"status": "occupied" }`)
				return
			}
		}
	}

	Write(resp, http.StatusOK, fmt.Sprintf(`{"status": "%s" }`, roomStatus.String()))
}

func (mm *MaestroGameRoomsManager) deregisterRoom(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id, ok := vars["roomID"]
	if !ok {
		Write(resp, http.StatusBadRequest, "roomID is missing in parameters")
		return
	}

	delete(mm.roomsMap, id)

	Write(resp, http.StatusOK, "room deregistered with success")
}

// Routes for management

func (mm *MaestroGameRoomsManager) setRoomStatus(resp http.ResponseWriter, req *http.Request) {
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

	mm.writeMap(id, gameRoomPingStatus)

	Write(resp, http.StatusOK, "status for room updated with success")
}

func (mm *MaestroGameRoomsManager) transitStatus(resp http.ResponseWriter, req *http.Request) {
	previousStatus := req.FormValue("previousStatus")
	newStatus := req.FormValue("newStatus")
	numberOfRooms := req.FormValue("numberOfRooms")

	previousStatusPing, err := FromStringToGameRoomPingStatus(previousStatus)
	if err != nil {
		Write(resp, http.StatusBadRequest, "previousStatus is not valid")
		return
	}

	newStatusPing, err := FromStringToGameRoomPingStatus(newStatus)
	if err != nil {
		Write(resp, http.StatusBadRequest, "newStatus is not valid")
		return
	}

	numberOfRoomsInt, err := strconv.Atoi(numberOfRooms)
	if err != nil {
		Write(resp, http.StatusBadRequest, "numberOfRooms is not valid")
		return
	}

	var roomsToUpdate []string

	mm.mutex.RLock()
	for id, status := range mm.roomsMap {
		if status == previousStatusPing {
			roomsToUpdate = append(roomsToUpdate, id)
			numberOfRoomsInt--
			if numberOfRoomsInt == 0 {
				break
			}
		}
	}
	mm.mutex.RUnlock()

	for _, id := range roomsToUpdate {
		mm.writeMap(id, newStatusPing)
	}

	Write(resp, http.StatusOK, "rooms status for room updated with success")
}

func (mm *MaestroGameRoomsManager) setFullAutoscaling(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	stringAutoscalingMode, ok := vars["autoscalingMode"]
	if !ok {
		Write(resp, http.StatusBadRequest, "autoscalingMode is missing in parameters")
		return
	}

	autoscalingMode, err := strconv.ParseBool(stringAutoscalingMode)
	if err != nil {
		Write(resp, http.StatusBadRequest, "autoscalingMode must be any type of boolean")
		return
	}

	mm.autoscalingMode = autoscalingMode

	Write(resp, http.StatusOK, fmt.Sprintf("Autoscaling mode updated to %v", autoscalingMode))
}

func (mm *MaestroGameRoomsManager) readMap(roomId string) (GameRoomPingStatus, bool) {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	status, ok := mm.roomsMap[roomId]

	return status, ok
}

func (mm *MaestroGameRoomsManager) readCreatedAtMap(roomId string) (time.Time, bool) {
	mm.mutex.RLock()
	defer mm.mutex.RUnlock()
	createdAt, ok := mm.roomsCreatedAtMap[roomId]

	return createdAt, ok
}

func (mm *MaestroGameRoomsManager) writeMap(roomId string, status GameRoomPingStatus) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	mm.roomsMap[roomId] = status
}

func (mm *MaestroGameRoomsManager) writeCreatedAtMap(roomId string) {
	mm.mutex.Lock()
	defer mm.mutex.Unlock()
	mm.roomsCreatedAtMap[roomId] = time.Now()
}

func main() {
	fmt.Printf("Starting Maestro's Game Rooms Manager\n")

	manager := MaestroGameRoomsManager{
		roomsMap: make(map[string]GameRoomPingStatus),
		mutex:    &sync.RWMutex{},
	}

	router := mux.NewRouter()

	// Routes for game rooms
	router.HandleFunc("/register/{roomID}", manager.registerRoomWithInitialStatus).Methods("POST")
	router.HandleFunc("/getStatus/{roomID}", manager.retrieveTargetStatusToRoom).Methods("GET")
	router.HandleFunc("/deregister/{roomID}", manager.deregisterRoom).Methods("DELETE")

	// Routes for management
	router.HandleFunc("/setStatus/{roomID}/{status}", manager.setRoomStatus).Methods("POST")
	router.HandleFunc("/transitStatus", manager.transitStatus).Methods("POST").Queries("previousStatus", "", "newStatus", "", "numberOfRooms", "")
	router.HandleFunc("/autoscalingMode", manager.setFullAutoscaling).Methods("POST").Queries("autoscalingMode", "")

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
