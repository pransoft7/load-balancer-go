package lb

import (
	"context"
	"log"
	"net"
	"sync/atomic"
	"time"
)

const failThresholdForBackend int32 = 3
const healthCheckInterval = 5 * time.Second

type Service struct {
	Address     string
	FailCounter atomic.Int32
	Healthy     atomic.Bool
}

type ServicePool struct {
	instances []*Service
	next      atomic.Uint64
}

func newServicePool(addresses []string) *ServicePool {
	var instances []*Service
	for _, a := range addresses {
		newService := &Service{Address: a}
		newService.Healthy.Store(true)
		instances = append(instances, newService)
	}
	return &ServicePool{
		instances: instances,
		next:      atomic.Uint64{},
	}
}

func (pool *ServicePool) nextHealthyInstance() *Service {
	total := len(pool.instances)
	for i := 0; i < total; i++ {
		index := pool.next.Add(1) - 1
		svc := pool.instances[int(index)%total]
		if svc.Healthy.Load() {
			return svc
		}
	}
	return nil
}

func (pool *ServicePool) healthCheck(ctx context.Context) {
	for {
		allHealthy := true
		for _, svc := range pool.instances {
			healthConn, err := net.DialTimeout("tcp", svc.Address, time.Millisecond*500)
			if err != nil {
				allHealthy = false
				if svc.Healthy.Load() && svc.FailCounter.Add(1) >= failThresholdForBackend {
					log.Println("Backend unhealthy:", svc.Address)
					svc.Healthy.Store(false)
				}
				continue
			}
			healthConn.Close()
			if !svc.Healthy.Load() {
				log.Println("backend recovered:", svc.Address)
			}
			svc.FailCounter.Store(0)
			svc.Healthy.Store(true)
		}

		interval := healthCheckInterval
		// Unhealthy backends reduce the health check time interval
		if !allHealthy {
			interval = interval / 5
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
