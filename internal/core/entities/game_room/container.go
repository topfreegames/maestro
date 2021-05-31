package game_room

type Container struct {
	Name            string
	Image           string
	ImagePullPolicy string
	Command         []string
	Environment     []ContainerEnvironment
	Requests        ContainerResources
	Limits          ContainerResources
	Ports           []ContainerPort
}

type ContainerEnvironment struct {
	Name  string
	Value string
}

type ContainerResources struct {
	Memory string
	CPU    string
}

type ContainerPort struct {
	Name     string
	Protocol string
	Port     int
	HostPort int
}
