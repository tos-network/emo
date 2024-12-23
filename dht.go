package emo

// Copyright 2024 Terminos Storage Protocol
// This file is part of the tos library.
//
// The tos library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The tos library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the tos library. If not, see <http://www.gnu.org/licenses/>.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"log"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/tos-network/emo/protocol"
	"golang.org/x/crypto/sha3"
	"golang.org/x/exp/rand"
	"golang.org/x/net/ipv4"
	"golang.org/x/sys/unix"
)

// DHT represents the distributed hash table
type DHT struct {
	// config used for the dht
	config *Config
	// storage for values that saved to this node
	storage Storage
	// routing table that stores routing information about the network
	routing *routingTable
	// cache that tracks requests sent to other nodes
	cache *cache
	// manages fragmented packets that are larger than MTU
	packet *packetManager
	// udp listeners that are handling requests to/from other nodes
	listeners []*listener
	// latency router for finding the best routes
	latencyRouter *latencyRouter
	// pool of flatbuffer builder bufs to use when sending requests
	pool sync.Pool
	// the current listener to use when sending data
	cl int32
	// wait group for the dht
	wg sync.WaitGroup
	// for shutting down the dht
	quit chan struct{}
	// for shutting down the dht
	closeOnce sync.Once
}

// New creates a new dht
func New(cfg *Config) (*DHT, error) {
	if cfg.LocalID == nil {
		cfg.LocalID = randomID()
	} else if len(cfg.LocalID) != KEY_BYTES {
		return nil, errors.New("node id length is incorrect")
	}

	if int(cfg.Timeout) == 0 {
		cfg.Timeout = time.Minute
	}

	if cfg.Listeners < 1 {
		cfg.Listeners = runtime.GOMAXPROCS(0)
	}

	if cfg.SocketBufferSize < 1 {
		cfg.SocketBufferSize = 32 * 1024 * 1024
	}

	if cfg.SocketBatchSize < 1 {
		cfg.SocketBatchSize = 1024
	}

	if cfg.SocketBatchInterval < 1 {
		cfg.SocketBatchInterval = time.Millisecond
	}

	if cfg.Storage == nil {
		storage, err := InitializeStorage(cfg)
		if err != nil {
			return nil, err
		}
		cfg.Storage = storage
	}

	addr, err := net.ResolveUDPAddr("udp", cfg.ListenAddress)
	if err != nil {
		return nil, err
	}

	n := &node{
		id:        cfg.LocalID,
		address:   addr,
		latency:   time.Duration(0),
		failCount: 0,
		testMode:  false,
	}

	d := &DHT{
		config:  cfg,
		routing: newRoutingTable(n),
		cache:   newCache(cfg.Timeout),
		storage: cfg.Storage,
		packet:  newPacketManager(),
		quit:    make(chan struct{}),
		pool: sync.Pool{
			New: func() any {
				return flatbuffers.NewBuilder(1024)
			},
		},
	}
	d.latencyRouter = NewLatencyRouter(d)

	// start the udp listeners
	err = d.listen()
	if err != nil {
		return nil, err
	}

	// add the local node to our own routing table
	d.routing.insert(n.id, addr, 0, false)

	br := make(chan error, len(cfg.BootstrapAddresses))
	bn := make([]*node, len(cfg.BootstrapAddresses))

	for i := range cfg.BootstrapAddresses {
		addr, err := net.ResolveUDPAddr("udp", cfg.BootstrapAddresses[i])
		if err != nil {
			return nil, err
		}

		bn[i] = &node{address: addr}
	}

	// TODO : this should be a recursive lookup, use journey
	d.findNodes(bn, cfg.LocalID, func(err error) {
		br <- err
	})

	var successes int

	for range cfg.BootstrapAddresses {
		err := <-br
		if err != nil {
			log.Printf("bootstrap failed: %s\n", err.Error())
			continue
		}
		successes++
	}

	if successes < 1 && len(cfg.BootstrapAddresses) > 1 {
		return nil, errors.New("bootstrapping failed")
	}

	// Start the peer refresh process
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.refreshPeers()
	}()

	// Add WaitGroup for refreshKeys goroutine
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.refreshKeys()
	}()

	return d, nil
}

func (d *DHT) listen() error {
	for i := 0; i < d.config.Listeners; i++ {
		cfg := net.ListenConfig{
			Control: control,
		}

		// start one of several listeners
		c, err := cfg.ListenPacket(context.Background(), "udp", d.config.ListenAddress)
		if err != nil {
			return err
		}

		err = c.(*net.UDPConn).SetReadBuffer(d.config.SocketBufferSize)
		if err != nil {
			return err
		}

		err = c.(*net.UDPConn).SetWriteBuffer(d.config.SocketBufferSize)
		if err != nil {
			return err
		}

		l := &listener{
			conn:       ipv4.NewPacketConn(c),
			routing:    d.routing,
			cache:      d.cache,
			storage:    d.storage,
			packet:     d.packet,
			buffer:     flatbuffers.NewBuilder(65527),
			localID:    d.config.LocalID,
			timeout:    d.config.Timeout,
			logging:    d.config.Logging,
			bufferSize: d.config.SocketBufferSize,
			writeBatch: make([]ipv4.Message, d.config.SocketBatchSize),
			readBatch:  make([]ipv4.Message, d.config.SocketBatchSize),
			ftimer:     time.NewTicker(d.config.SocketBatchInterval),
			quit:       make(chan struct{}),
		}

		for i := range l.writeBatch {
			l.readBatch[i].Buffers = [][]byte{make([]byte, 1500)}
			l.writeBatch[i].Buffers = [][]byte{make([]byte, 1500)}
		}

		d.wg.Add(2)
		go func() {
			defer d.wg.Done()
			l.flusher()
		}()
		go func() {
			defer d.wg.Done()
			l.process()
		}()

		d.listeners = append(d.listeners, l)
	}

	d.wg.Add(1)
	// monitor the routing table for stale nodes
	go d.monitor()

	return nil
}

// Store a value on the network. If the value fails to store, the provided callback will be returned with the error
func (d *DHT) Store(key, value []byte, ttl time.Duration, callback func(err error)) {
	if len(key) != KEY_BYTES {
		callback(errors.New("key must be 20 bytes in length"))
		return
	}

	// value must be smaller than 32 kb
	if len(value) > VALUE_BYTES {
		callback(errors.New("value must be less than 32kb in length"))
		return
	}

	// TODO  use NTP time for this?
	created := time.Now()

	v := []*Value{
		{
			Key:     key,
			Value:   value,
			TTL:     ttl,
			Created: created,
		},
	}

	// get the k closest nodes to store the value to
	ns := d.routing.closestN(key, K)

	if len(ns) < 1 {
		callback(errors.New("no nodes found"))
		return
	}

	// track the number of successful stores we've had from each node
	// before calling the user provided callback
	var r int32

	// get a spare buffer to generate our requests with
	buf := d.pool.Get().(*flatbuffers.Builder)
	defer d.pool.Put(buf)

	for _, n := range ns {
		// shortcut the request if its to the local node
		if bytes.Equal(n.id, d.config.LocalID) {
			d.storage.Set(key, value, created, ttl)

			if len(ns) == 1 {
				// we're the only node, so call the callback immediately
				callback(nil)
				return
			}

			continue
		}

		// generate a new random request ID and event
		rid := pseudorandomID()
		req := eventStoreRequest(buf, rid, d.config.LocalID, v)

		// select the next listener to send our request
		err := d.listeners[(atomic.AddInt32(&d.cl, 1)-1)%int32(len(d.listeners))].request(
			n.address,
			rid,
			req,
			func(event *protocol.Event, err error) bool {
				// TODO : we call the user provided callback as soon as there's an error
				// ideally, we should consider the store a success if a minimum number of
				// nodes successfully managed to store the value
				if err != nil {
					callback(err)
					return true
				}

				if atomic.AddInt32(&r, 1) == int32(len(ns)-1) {
					// we've had the correct number of responses back, so lets call the
					// user provided callback with a success
					callback(nil)
				}

				return true
			},
		)

		if err != nil {
			// if we fail to write to the socket, send the error to the callback immediately
			callback(err)
			return
		}
	}
}

// Close shuts down the dht
func (d *DHT) Close() error {
	var closeErr error

	d.closeOnce.Do(func() {
		// Signal all goroutines to stop
		close(d.quit)

		// Close all listeners first
		for _, l := range d.listeners {
			l.Close()

			// Close the underlying connection
			if err := l.conn.Close(); err != nil {
				//log.Printf("Error closing listener: %v", err)
				closeErr = nil
			}

		}

		if d.storage != nil {
			if closer, ok := d.storage.(interface{ Close() error }); ok {
				if err := closer.Close(); err != nil {
					closeErr = err
				}
			}
		}
		// Wait for all goroutines to finish
		d.wg.Wait()
	})
	return closeErr
}

// Find finds a value on the network if it exists. If the key being queried has multiple values, the callback will be invoked for each result
// Any returned value will not be safe to use outside of the callback, so you should copy it if its needed elsewhere
func (d *DHT) Find(key []byte, callback func(value []byte, err error), opts ...*FindOption) {
	if len(key) != KEY_BYTES {
		callback(nil, errors.New("key must be 20 bytes in length"))
		return
	}

	var from time.Time

	// TODO do this properly...
	if len(opts) > 0 {
		from = opts[0].from
	}

	// we should check our own cache first before sending a request
	vs, ok := d.storage.Get(key, from)
	if ok {
		for i := range vs {
			callback(vs[i].Value, nil)
		}
		return
	}

	// a correct implementation should send mutiple requests concurrently,
	// but here we're only send a request to the closest node
	ns := d.routing.closestN(key, K)
	if len(ns) == 0 {
		callback(nil, errors.New("no nodes found"))
		return
	}

	// K iterations to find the key we want
	j := newJourney(d.config.LocalID, key, K)
	j.add(ns)

	// try lookup to best 3 nodes
	for _, n := range j.next(3) {
		// get a spare buffer to generate our requests with
		buf := d.pool.Get().(*flatbuffers.Builder)
		defer d.pool.Put(buf)

		// generate a new random request ID
		rid := pseudorandomID()
		req := eventFindValueRequest(buf, rid, d.config.LocalID, key, from)

		// select the next listener to send our request
		err := d.listeners[(atomic.AddInt32(&d.cl, 1)-1)%int32(len(d.listeners))].request(
			n.address,
			rid,
			req,
			d.findValueCallback(n.id, key, from, callback, j),
		)

		if err != nil {
			// if we fail to write to the socket, send the error to the callback immediately
			callback(nil, err)
			return
		}
	}
}

// TODO : this is all pretty garbage, refactor!
// return the callback used to handle responses to our findValue requests, tracking the number of requests we have made
func (d *DHT) findValueCallback(id, key []byte, from time.Time, callback func(value []byte, err error), j *journey) func(event *protocol.Event, err error) bool {
	return func(event *protocol.Event, err error) bool {
		if err != nil {
			if errors.Is(err, ErrRequestTimeout) {
				d.routing.remove(id)
			}
		}

		journeyCompleted, shouldError := j.responseReceived()

		if journeyCompleted {
			// ignore this response, we've already received what we've needed
			return true
		}

		// journey is completed, ignore this response
		if err != nil {
			// if there's an actual error, send that to the user
			if shouldError {
				callback(nil, err)
				return true
			}
			return false
		}

		payloadTable := new(flatbuffers.Table)

		if !event.Payload(payloadTable) {
			callback(nil, errors.New("invalid response to find value request"))
			return false
		}

		f := new(protocol.FindValue)
		f.Init(payloadTable.Bytes, payloadTable.Pos)

		// check if we received the value or if we received a list of closest
		// neighbours that might have the key
		if f.ValuesLength() > 0 {
			// TODO make this better
			j.addOutstanding(event.SenderBytes(), int(f.Found()))
			j.removeOutstanding(event.SenderBytes(), f.ValuesLength())

			for i := 0; i < f.ValuesLength(); i++ {
				vd := new(protocol.Value)

				if !f.Values(vd, i) {
					callback(nil, errors.New("bad find value data"))
					return false
				}

				if !j.seenValue(vd.ValueBytes()) {
					callback(vd.ValueBytes(), nil)
				}
			}

			// attempt to finish the journey
			return j.finish(false)
		} else if f.NodesLength() < 1 {
			// mark the journey as finished so no more
			// requests will be made
			if j.finish(false) {
				callback(nil, errors.New("value not found"))
				return true
			}

			return false
		}

		// collect the new nodes from the response
		newNodes := make([]*node, f.NodesLength())

		for i := 0; i < f.NodesLength(); i++ {
			nd := new(protocol.Node)

			if !f.Nodes(nd, i) {
				callback(nil, errors.New("bad find value node data"))
				return false
			}

			nad := &net.UDPAddr{
				IP:   make(net.IP, 4),
				Port: int(binary.LittleEndian.Uint16(nd.AddressBytes()[4:])),
			}

			copy(nad.IP, nd.AddressBytes()[:4])

			nid := make([]byte, KEY_BYTES)
			copy(nid, nd.IdBytes())

			newNodes[i] = &node{
				id:      id,
				address: nad,
			}
		}

		// add them to the journey and then get the next recommended routes to query
		j.add(newNodes)

		ns := j.next(3)
		if ns == nil {
			if j.finish(false) {
				callback(nil, errors.New("value not found"))
				return true
			}
			return false
		}

		// the key wasn't found, so send a request to the next node
		// get a spare buffer to generate our requests with
		buf := d.pool.Get().(*flatbuffers.Builder)
		defer d.pool.Put(buf)

		for _, n := range ns {
			// generate a new random request ID
			rid := pseudorandomID()
			req := eventFindValueRequest(buf, rid, d.config.LocalID, key, from)

			// select the next listener to send our request
			err := d.listeners[(atomic.AddInt32(&d.cl, 1)-1)%int32(len(d.listeners))].request(
				n.address,
				rid,
				req,
				d.findValueCallback(n.id, key, from, callback, j),
			)

			if err != nil {
				// if we fail to write to the socket, send the error to the callback immediately
				if j.finish(false) {
					callback(nil, err)
					return true
				}
			}
		}

		return false
	}
}

func (d *DHT) findNodes(ns []*node, target []byte, callback func(err error)) {
	// create the journey here, but don't add the bootstrap
	// node as we don't know it's id yet
	j := newJourney(d.config.LocalID, target, K)

	// get a spare buffer to generate our requests with
	buf := d.pool.Get().(*flatbuffers.Builder)
	defer d.pool.Put(buf)

	for _, n := range ns {
		// generate a new random request ID and event
		rid := pseudorandomID()
		req := eventFindNodeRequest(buf, rid, d.config.LocalID, target)

		// select the next listener to send our request
		err := d.listeners[(atomic.AddInt32(&d.cl, 1)-1)%int32(len(d.listeners))].request(
			n.address,
			rid,
			req,
			d.findNodeCallback(target, callback, j),
		)

		if err != nil {
			// if we fail to write to the socket, send the error to the callback immediately
			callback(err)
			return
		}
	}
}

func (d *DHT) findNodeCallback(target []byte, callback func(err error), j *journey) func(*protocol.Event, error) bool {
	return func(event *protocol.Event, err error) bool {
		_, shouldError := j.responseReceived()

		// journey is completed, ignore this response
		if err != nil {
			// if there's an actual error, send that to the user
			if shouldError {
				callback(err)
				return true
			}
			return false
		}

		payloadTable := new(flatbuffers.Table)

		if !event.Payload(payloadTable) {
			callback(errors.New("invalid response to find node request"))
			return false
		}

		f := new(protocol.FindNode)
		f.Init(payloadTable.Bytes, payloadTable.Pos)

		newNodes := make([]*node, f.NodesLength())

		for i := 0; i < f.NodesLength(); i++ {
			fn := new(protocol.Node)

			if f.Nodes(fn, i) {
				nad := &net.UDPAddr{
					IP:   make(net.IP, 4),
					Port: int(binary.LittleEndian.Uint16(fn.AddressBytes()[4:])),
				}

				copy(nad.IP, fn.AddressBytes()[:4])

				// create a copy of the node id
				nid := make([]byte, fn.IdLength())
				copy(nid, fn.IdBytes())

				d.routing.insert(nid, nad, time.Duration(0), false)

				newNodes[i] = &node{
					id:      nid,
					address: nad,
				}
			}
		}

		j.add(newNodes)

		ns := j.next(3)
		if ns == nil {
			// we've completed our search of nodes
			if j.finish(false) {
				callback(nil)
				return true
			}
			return false
		}

		buf := d.pool.Get().(*flatbuffers.Builder)
		defer d.pool.Put(buf)

		for _, n := range ns {
			// generate a new random request ID and event
			rid := pseudorandomID()
			req := eventFindNodeRequest(buf, rid, d.config.LocalID, target)

			// select the next listener to send our request
			err := d.listeners[(atomic.AddInt32(&d.cl, 1)-1)%int32(len(d.listeners))].request(
				n.address,
				rid,
				req,
				d.findNodeCallback(target, callback, j),
			)

			if err != nil {
				// if we fail to write to the socket, send the error to the callback immediately
				callback(err)
				return false
			}
		}

		return false
	}
}

// monitors peers on the network and sends them ping requests
func (d *DHT) monitor() {
	defer d.wg.Done()
	ticker := time.NewTicker(time.Hour / 2)
	defer ticker.Stop()

	for {
		select {
		case <-d.quit:
			// Shutdown signal received, exit the loop
			return
		case <-ticker.C:
			// Existing monitoring logic
			now := time.Now()

			var nodes []*node

			for i := 0; i < KEY_BITS; i++ {
				d.routing.buckets[i].iterate(func(n *node) {
					// If we haven't seen this node recently, add it to the list to ping
					if n.seen.Add(time.Hour / 2).After(now) {
						nodes = append(nodes, n)
					}
				})
			}

			// Send ping requests to nodes
			buf := d.pool.Get().(*flatbuffers.Builder)

			for _, n := range nodes {
				// Send a ping to each node to check if it's still alive
				rid := pseudorandomID()
				req := eventPing(buf, rid, d.config.LocalID)

				err := d.listeners[(atomic.AddInt32(&d.cl, 1)-1)%int32(len(d.listeners))].request(
					n.address,
					rid,
					req,
					func(event *protocol.Event, err error) bool {
						if err != nil {
							if errors.Is(err, ErrRequestTimeout) {
								d.routing.remove(n.id)
							} else {
								log.Println(err)
							}
						} else {
							d.routing.seen(n.id)
						}
						return true
					},
				)

				if err != nil {
					if errors.Is(err, net.ErrClosed) {
						// Connection is closed, exit the monitor
						return
					}
				}
			}

			d.pool.Put(buf)
		}
	}
}

// Key creates a new 32 byte key hasehed with Keccak256 from a string, byte slice or int
func Key(k any) []byte {
	var h [32]byte
	hasher := sha3.NewLegacyKeccak256()
	switch key := k.(type) {
	case string:
		hasher.Write([]byte(key))
		hasher.Sum(h[:0])
	case []byte:
		hasher.Write(key)
		hasher.Sum(h[:0])
	case int:
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(key))
		hasher.Write(b)
		hasher.Sum(h[:0])
	default:
		panic("unsupported key type!")
	}

	return h[:]
}

// "borrow" this from github.com/libp2p/go-reuseport as we don't care about other operating systems right now :)
func control(network, address string, c syscall.RawConn) error {
	var err error

	c.Control(func(fd uintptr) {
		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if err != nil {
			return
		}

		err = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		if err != nil {
			return
		}
	})

	return err
}

func (d *DHT) refreshPeers() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-d.quit:
			// Shutdown signal received, exit the loop
			return
		case <-ticker.C:
			// Perform the peer refresh
			d.refreshBuckets()
		}
	}
}

func (d *DHT) refreshBuckets() {
	for i := 0; i < KEY_BITS; i++ {
		bucket := &d.routing.buckets[i]
		bucket.refresh(d)
	}
}

func (d *DHT) pingNode(n *node) bool {
	response := make(chan bool, 1)

	rid := pseudorandomID()
	buf := d.pool.Get().(*flatbuffers.Builder)
	defer d.pool.Put(buf)

	req := eventPing(buf, rid, d.config.LocalID)

	err := d.listeners[(atomic.AddInt32(&d.cl, 1)-1)%int32(len(d.listeners))].request(
		n.address,
		rid,
		req,
		func(event *protocol.Event, err error) bool {
			if err != nil {
				response <- false
			} else {
				d.routing.seen(n.id) //update the node's last seen time
				response <- true
			}
			return true
		},
	)

	if err != nil {
		return false
	}

	select {
	case res := <-response:
		return res
	case <-time.After(d.config.Timeout):
		return false
	}
}

func (d *DHT) generateRandomIDInBucket(b *bucket) []byte {
	// Get the index of the bucket
	bucketIndex := d.routing.getBucketIndex(b)

	// Copy the local ID up to the prefix length
	id := make([]byte, KEY_BYTES)
	copy(id, d.config.LocalID)

	// Flip the (bucketIndex + 1)-th bit
	byteIndex := bucketIndex / 8
	bitIndex := bucketIndex % 8
	id[byteIndex] ^= 1 << (7 - bitIndex)

	// Randomize the remaining bits
	for i := byteIndex + 1; i < KEY_BYTES; i++ {
		id[i] = byte(rand.Intn(256))
	}

	return id
}

func (d *DHT) lookup(targetID []byte) []*node {
	// Initialize the shortlist with the closest nodes known
	//shortlist := d.routing.closestN(targetID, ALPHA)
	shortlist := d.latencyRouter.GetBestRoutes(targetID, ALPHA)
	// Map to track queried nodes
	queried := make(map[string]bool)

	// Iterative lookup
	for {
		// Find unqueried nodes
		unqueried := []*node{}
		for _, n := range shortlist {
			key := n.address.String()
			if !queried[key] {
				unqueried = append(unqueried, n)
			}
		}

		if len(unqueried) == 0 || len(queried) >= K {
			break
		}

		// Query up to ALPHA nodes in parallel
		for _, n := range unqueried[:min(ALPHA, len(unqueried))] {
			queried[n.address.String()] = true
			// Send FIND_NODE request to n
			// Handle responses and update shortlist
		}
	}

	return shortlist[:min(K, len(shortlist))]
}

func (d *DHT) refreshKeys() {
	ticker := time.NewTicker(time.Hour / 2) // Refresh every 30 minutes
	defer ticker.Stop()

	for {
		select {
		case <-d.quit:
			return
		case <-ticker.C:
			// Get all stored keys
			var keys [][]byte
			d.storage.Iterate(func(value *Value) bool {
				keys = append(keys, value.Key)
				return true
			})

			// Refresh each key
			for _, key := range keys {
				value, exists := d.storage.Get(key, time.Now())
				if !exists || len(value) == 0 {
					continue
				}

				// Re-store the value with the remaining TTL
				remainingTTL := time.Until(value[0].expires)
				if remainingTTL > 0 {
					d.Store(key, value[0].Value, remainingTTL, func(err error) {
						if err != nil {
							log.Printf("Failed to refresh key %x: %v", key, err)
						}
					})
				}
			}
		}
	}
}

func (d *DHT) findWithLatency(key []byte, callback func([]byte, error)) {
	router := NewLatencyRouter(d)
	bestNodes := router.GetBestRoutes(key, 3) // Get top 3 lowest latency nodes

	if len(bestNodes) == 0 {
		callback(nil, errors.New("no suitable nodes found"))
		return
	}

	// Use existing journey mechanism with latency-optimized nodes
	j := newJourney(d.config.LocalID, key, K)
	j.add(bestNodes)

	// Continue with existing find logic...
}
