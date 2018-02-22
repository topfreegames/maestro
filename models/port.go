// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"k8s.io/client-go/pkg/api/v1"

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

//InitAvailablePorts add to freePorts set the range of available ports
func InitAvailablePorts(redisClient redisinterfaces.RedisClient, begin, end int) error {
	freePortsKey := FreePortsRedisKey()
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

//GetFreePorts pops n ports from freePorts set, adds them to takenPorts set and return in an array
func GetFreePorts(redisClient redisinterfaces.RedisClient, n int) ([]int, error) {
	if n <= 0 {
		return []int{}, nil
	}
	//TODO: check if enough ports
	//TODO: check if port is available in host machine

	freePortsKey := FreePortsRedisKey()
	pipe := redisClient.TxPipeline()
	ports := make([]int, n)
	portsCmd := make([]*redis.StringCmd, n)
	for i := 0; i < n; i++ {
		portsCmd[i] = pipe.SPop(freePortsKey)
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
func RetrievePorts(redisClient redisinterfaces.RedisClient, ports []*Port) error {
	if ports == nil || len(ports) == 0 {
		return nil
	}
	freePortsKey := FreePortsRedisKey()
	pipe := redisClient.TxPipeline()
	for _, port := range ports {
		pipe.SAdd(freePortsKey, port.HostPort)
	}
	_, err := pipe.Exec()
	return err
}

//RetrieveV1Ports s a wrapper of RetrievePorts by extracting HostPort from each element of a list of k8s.io/client-go/pkg/api/v1.ContainerPort
func RetrieveV1Ports(redisClient redisinterfaces.RedisClient, v1ports []v1.ContainerPort) error {
	if v1ports == nil || len(v1ports) == 0 {
		return nil
	}
	ports := make([]*Port, len(v1ports))
	for i, port := range v1ports {
		ports[i] = &Port{
			HostPort: int(port.HostPort),
		}
	}
	return RetrievePorts(redisClient, ports)
}
