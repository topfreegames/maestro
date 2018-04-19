package models

import (
	"fmt"

	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
)

// PortRange represents the port range that maestro can use
// to give ports to a scheduler.
type PortRange struct {
	Start int
	End   int
}

// IsSet returns true if the start port and end port are valids
func (p *PortRange) IsSet() bool {
	return p != nil && p.Start > 0 && p.End > 0
}

// IsValid returns trus if port range is valid
func (p *PortRange) IsValid() bool {
	return p.IsSet() && p.Start < p.End
}

// Equals returns true if port ranges are equal
func (p *PortRange) Equals(pr *PortRange) bool {
	if p == nil && pr == nil {
		return true
	}

	if p == nil || pr == nil {
		return false
	}

	return p.Start == pr.Start && p.End == pr.End
}

// PortIsInRange returns true if port is between start and end
func (p *PortRange) PortIsInRange(port int32) bool {
	portInt := int(port)
	return portInt >= p.Start && portInt <= p.End
}

// PopulatePool starts the set on redis with ports from start to end
func (p *PortRange) PopulatePool(redis redisinterfaces.RedisClient, schedulerName string) error {
	setKey := FreeSchedulerPortsRedisKey(schedulerName)

	pipe := redis.TxPipeline()
	pipe.Del(setKey)
	_, err := pipe.Exec()
	// continue even if an error occurrs

	err = InitAvailablePorts(redis, FreeSchedulerPortsRedisKey(schedulerName), p.Start, p.End)

	return err
}

// HasIntersection returns true if the port ranges have intersection with each other.
func (p *PortRange) HasIntersection(pr *PortRange) bool {
	p1, p2 := p, pr

	if p1 == nil || p2 == nil {
		return false
	}

	case1 := p2.Start <= p1.Start && p1.Start <= p2.End
	case2 := p2.Start <= p1.End && p1.End <= p2.End

	return case1 || case2
}

func (p *PortRange) String() string {
	if p == nil {
		return "empty"
	}

	return fmt.Sprintf("%d-%d", p.Start, p.End)
}
