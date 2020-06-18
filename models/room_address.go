// maestro
// https://github.com/topfreegames/maestro
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	redisinterfaces "github.com/topfreegames/extensions/redis/interfaces"
	maestroErrors "github.com/topfreegames/maestro/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RoomAddressesFromHostPort is the struct that defines room addresses in production (using node HostPort)
type RoomAddressesFromHostPort struct {
	logger                  logrus.FieldLogger
	useCache                bool
	cacheExpirationInterval time.Duration
	ipv6KubernetesLabelKey  string
}

// NewRoomAddressesFromHostPort is the RoomAddressesFromHostPort constructor
func NewRoomAddressesFromHostPort(
	logger logrus.FieldLogger, ipv6KubernetesLabelKey string,
	useCache bool, cacheExpirationInterval time.Duration,
) *RoomAddressesFromHostPort {
	return &RoomAddressesFromHostPort{
		ipv6KubernetesLabelKey:  ipv6KubernetesLabelKey,
		useCache:                useCache,
		cacheExpirationInterval: cacheExpirationInterval,
		logger:                  logger,
	}
}

// Get gets room public addresses
func (r *RoomAddressesFromHostPort) Get(
	room *Room,
	kubernetesClient kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
) (*RoomAddresses, error) {
	if addrs := r.fromCache(redisClient, room); addrs != nil {
		return addrs, nil
	}
	addrs, err := getRoomAddresses(false, r.ipv6KubernetesLabelKey, room, kubernetesClient, redisClient, mr)
	if err != nil {
		return nil, err
	}
	r.toCache(redisClient, room, addrs)
	return addrs, nil
}

func (r RoomAddressesFromHostPort) toCache(
	redisClient redisinterfaces.RedisClient, room *Room, addrs *RoomAddresses,
) {
	if !r.useCache {
		return
	}
	l := r.logger.WithFields(logrus.Fields{
		"struct": "RoomAddressesFromNodePort",
		"method": "toCache",
	})
	b, err := json.Marshal(addrs)
	if err != nil {
		l.WithError(err).Error("failed to marshal room addresses")
		return
	}
	if redisClient.Set(r.buildCacheKey(room), b, r.cacheExpirationInterval).Err() != nil {
		l.WithError(err).Error("failed to set room addresses in redis")
	}
}

func (r RoomAddressesFromHostPort) fromCache(redisClient redisinterfaces.RedisClient, room *Room) *RoomAddresses {
	if !r.useCache {
		return nil
	}
	cmd := redisClient.Get(r.buildCacheKey(room))
	err := cmd.Err()
	l := r.logger.WithFields(logrus.Fields{
		"struct": "RoomAddressesFromNodePort",
		"method": "fromCache",
	})
	if err != nil && err != redis.Nil {
		l.WithError(err).Error("failed to get from redis")
		return nil
	}
	if err == redis.Nil {
		return nil
	}
	addrs := &RoomAddresses{}
	b, err := cmd.Bytes()
	if err != nil {
		l.WithError(err).Error("failed to cmd.Bytes()")
		return nil
	}
	if err := json.Unmarshal(b, addrs); err != nil {
		l.WithError(err).Error("failed unmarshal room addresses from redis")
		return nil
	}
	return addrs
}

func (r *RoomAddressesFromHostPort) buildCacheKey(room *Room) string {
	return fmt.Sprintf("room-addr-%s-%s", room.SchedulerName, room.ID)
}

// RoomAddressesFromNodePort is the struct that defines room addresses in development (using NodePort service)
type RoomAddressesFromNodePort struct {
	logger                  logrus.FieldLogger
	useCache                bool
	cacheExpirationInterval time.Duration
	ipv6KubernetesLabelKey  string
}

// NewRoomAddressesFromNodePort is the RoomAddressesFromNodePort constructor
func NewRoomAddressesFromNodePort(
	logger logrus.FieldLogger, ipv6KubernetesLabelKey string,
	useCache bool, cacheExpirationInterval time.Duration,
) *RoomAddressesFromNodePort {
	return &RoomAddressesFromNodePort{
		ipv6KubernetesLabelKey:  ipv6KubernetesLabelKey,
		useCache:                useCache,
		cacheExpirationInterval: cacheExpirationInterval,
		logger:                  logger,
	}
}

// Get gets room public addresses
func (r *RoomAddressesFromNodePort) Get(
	room *Room, kubernetesClient kubernetes.Interface, redisClient redisinterfaces.RedisClient, mr *MixedMetricsReporter,
) (*RoomAddresses, error) {
	if addrs := r.fromCache(redisClient, room); addrs != nil {
		return addrs, nil
	}
	addrs, err := getRoomAddresses(true, r.ipv6KubernetesLabelKey, room, kubernetesClient, redisClient, mr)
	if err != nil {
		return nil, err
	}
	r.toCache(redisClient, room, addrs)
	return addrs, nil
}

func (r RoomAddressesFromNodePort) toCache(
	redisClient redisinterfaces.RedisClient, room *Room, addrs *RoomAddresses,
) {
	if !r.useCache {
		return
	}
	l := r.logger.WithFields(logrus.Fields{
		"struct": "RoomAddressesFromNodePort",
		"method": "toCache",
	})
	b, err := json.Marshal(addrs)
	if err != nil {
		l.WithError(err).Error("failed to marshal room addresses")
		return
	}
	if redisClient.Set(r.buildCacheKey(room), b, r.cacheExpirationInterval).Err() != nil {
		l.WithError(err).Error("failed to set room addresses in redis")
	}
}

func (r RoomAddressesFromNodePort) fromCache(redisClient redisinterfaces.RedisClient, room *Room) *RoomAddresses {
	if !r.useCache {
		return nil
	}
	cmd := redisClient.Get(r.buildCacheKey(room))
	err := cmd.Err()
	l := r.logger.WithFields(logrus.Fields{
		"struct": "RoomAddressesFromNodePort",
		"method": "fromCache",
	})
	if err != nil && err != redis.Nil {
		l.WithError(err).Error("failed to get from redis")
		return nil
	}
	if err == redis.Nil {
		return nil
	}
	addrs := &RoomAddresses{}
	b, err := cmd.Bytes()
	if err != nil {
		l.WithError(err).Error("failed to cmd.Bytes()")
		return nil
	}
	if err := json.Unmarshal(b, addrs); err != nil {
		l.WithError(err).Error("failed unmarshal room addresses from redis")
		return nil
	}
	return addrs
}

func (r RoomAddressesFromNodePort) buildCacheKey(room *Room) string {
	return fmt.Sprintf("room-addr-%s-%s", room.SchedulerName, room.ID)
}

func getRoomAddresses(
	IsNodePort bool,
	ipv6KubernetesLabelKey string,
	room *Room,
	kubernetesClient kubernetes.Interface,
	redisClient redisinterfaces.RedisClient,
	mr *MixedMetricsReporter,
) (*RoomAddresses, error) {
	rAddresses := &RoomAddresses{}
	roomPod, err := GetPodFromRedis(redisClient, mr, room.ID, room.SchedulerName)
	if err != nil {
		return nil, err
	}

	if roomPod == nil {
		return nil, fmt.Errorf(`pod "%s" does not exist on redis podMap`, room.ID)
	}

	if len(roomPod.Spec.NodeName) == 0 {
		return rAddresses, nil
	}

	node, err := kubernetesClient.CoreV1().Nodes().Get(roomPod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// get IPv6 label from node
	if ipv6Label, ok := node.GetLabels()[ipv6KubernetesLabelKey]; ok {
		ipv6LabelBytes := base58.Decode(ipv6Label)
		rAddresses.Ipv6Label = string(ipv6LabelBytes)
	}

	if IsNodePort {
		for _, address := range node.Status.Addresses {
			if address.Type == v1.NodeInternalIP {
				rAddresses.Host = address.Address
				break
			}
		}
		if rAddresses.Host == "" {
			return nil, maestroErrors.NewKubernetesError("no host found", errors.New("no node found to host room"))
		}

		roomSvc, err := kubernetesClient.CoreV1().Services(room.SchedulerName).Get(room.ID, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		for _, port := range roomSvc.Spec.Ports {
			if port.NodePort != 0 {
				rAddresses.Ports = append(rAddresses.Ports, &RoomPort{
					Name:     port.Name,
					Port:     port.NodePort,
					Protocol: string(port.Protocol),
				})
			}
		}
	} else {
		for _, address := range node.Status.Addresses {
			if address.Type == v1.NodeExternalDNS {
				rAddresses.Host = address.Address
				break
			}
		}
		if rAddresses.Host == "" {
			return nil, maestroErrors.NewKubernetesError("no host found", errors.New("no node found to host room"))
		}
		for _, container := range roomPod.Spec.Containers {
			for _, port := range container.Ports {
				if port.HostPort != 0 {
					rAddresses.Ports = append(rAddresses.Ports, &RoomPort{
						Name:     port.Name,
						Port:     port.HostPort,
						Protocol: string(port.Protocol),
					})
				}
			}
		}
	}
	if len(rAddresses.Ports) == 0 {
		return nil, maestroErrors.NewKubernetesError("no ports found", errors.New("no node port found to host room"))
	}
	return rAddresses, nil
}
