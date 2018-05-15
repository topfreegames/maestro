// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"errors"
	"strconv"
	"strings"

	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
)

// Port has the port container port, protocol and hostPort which is binded to the host machine port
type Port struct {
	ContainerPort int    `yaml:"containerPort" json:"containerPort" valid:"int64,required"`
	Protocol      string `yaml:"protocol" json:"protocol" valid:"required"`
	Name          string `yaml:"name" json:"name" valid:"required"`
	HostPort      int    `yaml:"-" json:"-"`
}

// ThePortChooser chooses ports to allocate to pods
var ThePortChooser PortChooser = &RandomPortChooser{}

// GetPortRange returns the start and end ports
func GetPortRange(configYaml *ConfigYAML, redis redisinterfaces.RedisClient) (start, end int, err error) {
	if configYaml.PortRange.IsSet() {
		return configYaml.PortRange.Start, configYaml.PortRange.End, nil
	}

	return GetGlobalPortRange(redis)
}

// GetRandomPorts returns n random ports within scheduler limits
func GetRandomPorts(start, end, quantity int) []int {
	return ThePortChooser.Choose(start, end, quantity)
}

// GetGlobalPortRange returns the port range used by default by maestro worker
func GetGlobalPortRange(redisClient redisinterfaces.RedisClient) (start, end int, err error) {
	portsRange, err := redisClient.Get(GlobalPortsPoolKey).Result()
	if err != nil {
		return 0, 0, err
	}

	split := strings.Split(portsRange, "-")
	if len(split) != 2 {
		return 0, 0, errors.New("invalid ports range on redis")
	}

	startStr, endStr := split[0], split[1]

	start, err = strconv.Atoi(startStr)
	if err != nil {
		return
	}

	end, err = strconv.Atoi(endStr)
	if err != nil {
		return
	}

	return start, end, nil
}

func portIsInGlobalRange(start, end int, port int32) bool {
	portInt := int(port)
	return portInt >= start && portInt <= end
}
