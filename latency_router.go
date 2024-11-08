package emo

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/tos-network/emo/protocol"
)

const (
	defaultLatencyThreshold = 500 * time.Millisecond
	latencyCheckInterval    = 2 * time.Hour
)

type latencyRouter struct {
	dht       *DHT
	threshold time.Duration
}

func NewLatencyRouter(dht *DHT) *latencyRouter {
	lr := &latencyRouter{
		dht:       dht,
		threshold: defaultLatencyThreshold,
	}
	// Add to DHT's wait group before starting goroutine
	dht.wg.Add(1)
	go func() {
		defer dht.wg.Done()
		lr.startLatencyUpdates()
	}()

	return lr
}

func (lr *latencyRouter) measureNodeLatency(n *node) time.Duration {
	if n == nil || n.address == nil {
		return time.Hour
	}
	if n.testMode {
		return n.latency
	}

	start := time.Now()
	done := make(chan error, 1)

	// Use existing ping mechanism
	rid := pseudorandomID()
	buf := lr.dht.pool.Get().(*flatbuffers.Builder)
	defer lr.dht.pool.Put(buf)

	req := eventPing(buf, rid, lr.dht.config.LocalID)
	err := lr.dht.listeners[0].request(n.address, rid, req, func(event *protocol.Event, err error) bool {
		done <- err
		return true
	})

	if err != nil {
		atomic.AddInt32(&n.failCount, 1)
		return time.Hour
	}

	select {
	case err := <-done:
		if err != nil {
			atomic.AddInt32(&n.failCount, 1)
			return time.Hour
		}
		latency := time.Since(start)
		atomic.StoreInt32(&n.failCount, 0)
		return latency
	case <-time.After(lr.threshold):
		atomic.AddInt32(&n.failCount, 1)
		return time.Hour
	}
}

func (lr *latencyRouter) GetBestRoutes(target []byte, count int) []*node {
	nodes := lr.dht.routing.closestN(target, count*2)
	if len(nodes) == 0 {
		return nil
	}

	type nodeLatency struct {
		node    *node
		latency time.Duration
	}

	nodeLatencies := make([]nodeLatency, len(nodes))
	var wg sync.WaitGroup

	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, node *node) {
			defer wg.Done()
			latency := lr.measureNodeLatency(node)
			nodeLatencies[idx] = nodeLatency{node, latency}
		}(i, n)
	}
	wg.Wait()

	// Sort by latency and filter out high-latency nodes
	sort.Slice(nodeLatencies, func(i, j int) bool {
		return nodeLatencies[i].latency < nodeLatencies[j].latency
	})

	result := make([]*node, 0, count)
	for i := 0; i < count && i < len(nodeLatencies); i++ {
		if nodeLatencies[i].latency < lr.threshold {
			result = append(result, nodeLatencies[i].node)
		}
	}

	return result
}

func (lr *latencyRouter) startLatencyUpdates() {
	ticker := time.NewTicker(latencyCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lr.dht.quit:
			return
		case <-ticker.C:
			lr.updateAllNodeLatencies()
		}
	}
}

func (lr *latencyRouter) updateAllNodeLatencies() {
	for i := 0; i < KEY_BITS; i++ {
		lr.dht.routing.buckets[i].iterate(func(n *node) {
			latency := lr.measureNodeLatency(n)
			n.latency = latency
		})
	}
}
