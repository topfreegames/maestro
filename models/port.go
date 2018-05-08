// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/api/core/v1"

	redis "github.com/go-redis/redis"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
)

// Port has the port container port, protocol and hostPort which is binded to the host machine port
type Port struct {
	ContainerPort int    `yaml:"containerPort" json:"containerPort" valid:"int64,required"`
	Protocol      string `yaml:"protocol" json:"protocol" valid:"required"`
	Name          string `yaml:"name" json:"name" valid:"required"`
	HostPort      int    `yaml:"-" json:"-"`
}

//FreePortsRedisKey returns the free ports set key on redis
func FreePortsRedisKey() string {
	return "maestro:free:ports"
}

// FreeSchedulerPortsRedisKey return the free ports key of a scheduler
func FreeSchedulerPortsRedisKey(schedulerName string) string {
	return fmt.Sprintf("maestro:free:ports:%s", schedulerName)
}

//InitAvailablePorts add to freePorts set the range of available ports
func InitAvailablePorts(redisClient redisinterfaces.RedisClient, freePortsKey string, begin, end int) error {
	const initPortsScript = `
if redis.call("EXISTS", KEYS[1]) == 0 then
  for i=ARGV[1],ARGV[2] do
    redis.call("SADD", KEYS[1], i)
  end
end
return "OK"
`
	cmd := redisClient.Eval(initPortsScript, []string{freePortsKey}, begin, end)
	err := cmd.Err()
	return err
}

// GetFreePorts pops n ports from freePorts set, adds them to takenPorts set and return in an array
func GetFreePorts(redisClient redisinterfaces.RedisClient, n int, portsPoolKey string) ([]int, error) {
	if n <= 0 {
		return []int{}, nil
	}
	//TODO: check if enough ports
	//TODO: check if port is available in host machine

	pipe := redisClient.TxPipeline()
	ports := make([]int, n)
	portsCmd := make([]*redis.StringCmd, n)
	for i := 0; i < n; i++ {
		portsCmd[i] = pipe.SPop(portsPoolKey)
	}
	_, err := pipe.Exec()
	if err != nil {
		return nil, err
	}
	for i, portCmd := range portsCmd {
		port64, err := portCmd.Int64()
		if err != nil {
			return nil, err
		}
		ports[i] = int(port64)
	}
	return ports, nil
}

//RetrievePorts gets a list of used ports, pops them from takenPorts set and adds to freePorts set so they can be used again
func RetrievePorts(redisClient redisinterfaces.RedisClient, ports []*Port, portsPoolKey string) error {
	if ports == nil || len(ports) == 0 {
		return nil
	}
	pipe := redisClient.TxPipeline()
	for _, port := range ports {
		pipe.SAdd(portsPoolKey, port.HostPort)
	}
	_, err := pipe.Exec()
	return err
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

//RetrieveV1Ports s a wrapper of RetrievePorts by extracting HostPort from each element of a list of k8s.io/api/core/v1.ContainerPort
func RetrieveV1Ports(redisClient redisinterfaces.RedisClient, v1ports []v1.ContainerPort, c *ConfigYAML) error {
	if v1ports == nil || len(v1ports) == 0 {
		return nil
	}

	start, end, err := GetGlobalPortRange(redisClient)
	if err != nil {
		return err
	}

	ports := []*Port{}
	for _, port := range v1ports {
		isInConfigRange := c.PortRange.IsSet() && c.PortRange.PortIsInRange(port.HostPort)
		isInGlobalRange := !c.PortRange.IsSet() && portIsInGlobalRange(start, end, port.HostPort)

		if isInConfigRange || isInGlobalRange {
			ports = append(ports, &Port{
				HostPort: int(port.HostPort),
			})
		}
	}
	return RetrievePorts(redisClient, ports, c.PortsPoolRedisKey())
}
