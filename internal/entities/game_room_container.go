package entities

type GameRoomContainer struct {
	Name            string
	Image           string
	ImagePullPolicy string
	Command         []string
	Environment     []GameRoomContainerEnvironment
	Requests        GameRoomContainerResources
	Limits          GameRoomContainerResources
	Ports           []GameRoomContainerPort
}

type GameRoomContainerEnvironment struct {
	Name  string
	Value string
}

type GameRoomContainerResources struct {
	Memory string
	CPU    string
}

type GameRoomContainerPort struct {
	Name     string
	Protocol string
	Port     int
	HostPort int
}
