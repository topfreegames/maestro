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
	if err := k.Load(env.Provider("MAESTRO_", ".", parseEnv), nil); err != nil {
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
		strings.TrimPrefix(strings.TrimPrefix(s, "MAESTRO_"), "SIMULATION_")), "_", ".", -1)
}
