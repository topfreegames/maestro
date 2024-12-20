package core

import (
	"log"
	"strings"
	"time"

	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

type RoomConfig struct {
	ID string `koanf:"id"`
}

type SchedulerConfig struct {
	Name string `koanf:"name"`
}

type Config struct {
	SyncInterval  time.Duration   `koanf:"syncinterval"`
	InitialStatus string          `koanf:"initialstatus"`
	URL           string          `koanf:"url"`
	Room          RoomConfig      `koanf:"room"`
	Scheduler     SchedulerConfig `koanf:"scheduler"`
}

func NewDefaultConfig() *Config {
	return &Config{
		SyncInterval:  5 * time.Second,
		InitialStatus: "unready",
	}
}

func LoadConfig() *Config {
	k := koanf.New(".")

	_ = k.Load(structs.Provider(NewDefaultConfig(), "room"), nil)

	// This is to load MAESTRO_* environment variables
	if err := k.Load(env.Provider("", ".", parseEnv), nil); err != nil {
		log.Fatalf("error loading config from env: %v", err)
	}

	if err := k.Load(env.Provider("SIMULATION_", ".", parseEnv), nil); err != nil {
		log.Fatalf("error loading config from env: %v", err)
	}

	var config Config
	if err := k.Unmarshal("", &config); err != nil {
		log.Fatalf("error unmarshalling config: %v", err)
	}

	return &config
}

func parseEnv(s string) string {
	return strings.Replace(strings.ToLower(
		strings.TrimPrefix(s, "SIMULATION_")), "_", ".", -1)
}
