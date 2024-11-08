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
	"math/bits"
	"net"
	"sort"
	"time"
)

// routing table stores buckets of every known node on the network
type routingTable struct {
	localNode *node
	// buckets of nodes active in the routing table
	buckets []bucket
}

// newRoutingTable creates a new routing table
func newRoutingTable(localNode *node) *routingTable {
	buckets := make([]bucket, KEY_BITS)

	for i := range buckets {
		buckets[i].nodes = make([]*node, K)
	}

	return &routingTable{
		localNode: localNode,
		buckets:   buckets,
	}
}

// insert a node to its corresponding bucket
func (t *routingTable) insert(id []byte, address *net.UDPAddr,
	latency time.Duration, testMode bool) {
	t.buckets[bucketID(t.localNode.id, id)].insert(id, address, latency, testMode)
}

// updates the timestamp of a node to seen
// returns true if the node exists and false
// if the node needs to be inserted into the
// routing table
func (t *routingTable) seen(id []byte) bool {
	return t.buckets[bucketID(t.localNode.id, id)].seen(id)
}

// remove the node from the routing table
func (t *routingTable) remove(id []byte) {
	t.buckets[bucketID(t.localNode.id, id)].remove(id, true)
}

func (rt *routingTable) getBucketIndex(b *bucket) int {
	for i := 0; i < KEY_BITS; i++ {
		if &rt.buckets[i] == b {
			return i
		}
	}
	return -1
}

// finds the closest known node for a given key
func (t *routingTable) closest(id []byte) *node {
	offset := bucketID(t.localNode.id, id)

	// scan outwardly from our selected bucket until we find a
	// node that is close to the target key
	var i int
	var scanned int

	for {
		var cd int
		var cn *node

		if offset > -1 && offset < KEY_BITS {
			t.buckets[offset].iterate(func(n *node) {
				// find a node which has the most matching bits
				nd := distance(n.id, id)
				if cd == 0 || nd > cd {
					cd = nd
					cn = n
				}
			})

			if cn != nil {
				return cn
			}

			scanned++
		}

		if scanned >= KEY_BITS {
			break
		}

		if i%2 == 0 {
			offset = offset + i + 1
		} else {
			offset = offset - i - 1
		}

		i++
	}

	return nil
}

// finds the closest known nodes for a given key
func (t *routingTable) closestN(id []byte, count int) []*node {
	offset := bucketID(t.localNode.id, id)

	var nodes []*node

	// scan outwardly from our selected bucket until we find a
	// node that is close to the target key
	var i int
	var scanned int

	for {
		if offset > -1 && offset < KEY_BITS {
			t.buckets[offset].iterate(func(n *node) {
				nodes = append(nodes, n)
			})

			if len(nodes) >= count {
				break
			}

			scanned++
		}

		if scanned >= KEY_BITS {
			break
		}

		if i%2 == 0 {
			offset = offset + i + 1
		} else {
			offset = offset - i - 1
		}

		i++
	}

	sort.Slice(nodes, func(i, j int) bool {
		idst := distance(nodes[i].id, id)
		jdst := distance(nodes[j].id, id)
		return idst > jdst
	})

	if len(nodes) < count {
		return nodes
	}

	return nodes[:count]
}

// neighbours the total number of nodes known to us
func (r *routingTable) neighbours() int {
	var neighbours int

	for i := range r.buckets {
		r.buckets[i].mu.Lock()
		neighbours = neighbours + r.buckets[i].size
		r.buckets[i].mu.Unlock()
	}

	return neighbours
}

// bucketID gets the correct bucket id for a given node, based on it's xor distance from our node
func bucketID(localID, targetID []byte) int {
	pfx := distance(localID, targetID)

	d := (KEY_BITS - pfx)

	if d == 0 {
		return d
	}

	return d - 1
}

func distance(localID, targetID []byte) int {
	var pfx int

	// xor each byte and check for the number of 0 least significant bits
	for i := 0; i < KEY_BYTES; i++ {
		d := localID[i] ^ targetID[i]

		if d == 0 {
			// byte is all 0's, so we add all bits
			pfx = pfx + 8
		} else {
			// there are some differences with this byte, so get the number
			// of leading zero bits and add them to the prefix
			pfx = pfx + bits.LeadingZeros8(d)
			break
		}
	}

	return pfx
}
