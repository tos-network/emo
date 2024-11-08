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

package emo

import (
	"bytes"
	"net"
	"sync"
	"time"
)

type bucket struct {
	// the number of nodes in the bucket, excluding the promotion cache
	size int
	// the amout of time before a node is considered stale
	expiry time.Duration
	// nodes holds all active nodes on the network
	nodes []*node
	// cache holds all nodes that could be promoted to the bucket when
	// other nodes expire
	cache []*node
	mu    sync.Mutex
}

// inserts a node into the bucket. if the bucket
// is full, it will return false
func (b *bucket) insert(id []byte, address *net.UDPAddr,
	latency time.Duration, testMode bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// try to remove the node. If it exists in the bucket,
	// then update it and add it to the end of the list
	rn := b.remove(id, false)
	if rn != nil {
		rn.seen = time.Now()
		rn.latency = latency
		rn.testMode = testMode
		b.nodes[b.size] = rn
		b.size++
		return true
	}

	n := &node{
		id:        id,
		address:   address,
		latency:   latency,
		failCount: 0,
		testMode:  testMode,
	}

	// if the bucket is not full, add the new node to the end
	if !b.full() {
		n.seen = time.Now()
		b.nodes[b.size] = n
		b.size++

		return true
	}

	var si int
	var stale *node

	now := time.Now()

	// check for any stale entries
	for i := 0; i < b.size; i++ {
		en := b.nodes[i]

		if now.After(n.seen.Add(b.expiry)) {
			if stale == nil && en.pending > 1 {
				stale = en
				si = i
			} else if stale != nil && en.pending > stale.pending {
				stale = en
				si = i
			}
		}
	}

	// delete the stalest entry
	if stale != nil {
		copy(b.nodes[si:], b.nodes[si+1:])
		b.nodes[b.size] = n
		return true
	}

	// if there's no space in the bucket, we add the node to the promotion cache
	// so it can be added to the main node list when other nodes expire
	b.stash(n)

	return true
}

// gets a node by its id
func (b *bucket) get(nodeID []byte) *node {
	// check the main routing bucket
	for i := 0; i < b.size; i++ {
		if bytes.Equal(b.nodes[i].id, nodeID) {
			return b.nodes[i]
		}
	}

	// check the promotion cache
	for i := 0; i < len(b.cache); i++ {
		if bytes.Equal(b.cache[i].id, nodeID) {
			return b.cache[i]
		}
	}

	return nil
}

// iterates over each node in the bucket
func (b *bucket) iterate(fn func(n *node)) {
	b.mu.Lock()

	for i := 0; i < b.size; i++ {
		fn(b.nodes[i])
	}

	b.mu.Unlock()
}

// sets a node as recently seen by updating it's seen timestamp
// if it still exists in the bucket. this is called when a node has
// responded to a request
func (b *bucket) seen(nodeID []byte) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := b.get(nodeID)
	if n != nil {
		// todo improve the safety of this
		n.seen = time.Now()
		return true
	}

	return false
}

// removes a node and returns it if it exists
func (b *bucket) remove(nodeID []byte, lock bool) *node {
	if lock {
		b.mu.Lock()
		defer b.mu.Unlock()
	}

	for i := b.size - 1; i >= 0; i-- {
		if bytes.Equal(b.nodes[i].id, nodeID) {
			r := b.nodes[i]

			copy(b.nodes[i:], b.nodes[i+1:])
			b.size--

			return r
		}
	}

	return nil
}

// stash stashes a node in the promotion cache
func (b *bucket) stash(n *node) {
	for i := range b.cache {
		if bytes.Equal(b.cache[i].id, n.id) {
			b.cache[i].seen = time.Now()
			return
		}
	}

	// TODO : restrict the size of the cache and
	// evict the oldest members of this cache before
	// adding any new items. a circular buf would be ideal here

	b.cache = append(b.cache, n)
}

func (b *bucket) full() bool {
	return b.size == K
}

func (b *bucket) fillBucket(d *DHT) {
	if len(b.nodes) >= K {
		return
	}

	// Generate a random ID within the bucket's range
	targetID := d.generateRandomIDInBucket(b)

	// Perform a lookup to find new nodes
	nodes := d.lookup(targetID)

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, n := range nodes {
		if len(b.nodes) >= K {
			break
		}
		b.insert(n.id, n.address, n.latency, n.testMode)
	}
}

func (b *bucket) refresh(d *DHT) {
	b.mu.Lock()
	nodes := make([]*node, len(b.nodes))
	copy(nodes, b.nodes)
	b.mu.Unlock()

	for _, n := range nodes {
		if !d.pingNode(n) {
			// Node is unresponsive, remove it
			b.remove(n.id, true)
		}
	}

	// If the bucket is not full, find new nodes
	b.fillBucket(d)
}
