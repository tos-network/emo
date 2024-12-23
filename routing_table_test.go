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
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutingTableFindNearest(t *testing.T) {
	rt := newRoutingTable(&node{
		id: randomID(),
	})

	// generate a random target key we want to look up
	target := randomID()

	// attempt to search an empty routing table
	n := rt.closest(target)
	require.Nil(t, n)

	// insert 10000 nodes into the routing table
	for i := 0; i < 10000; i++ {
		rt.insert(randomID(), nil, time.Duration(0), false)
	}

	// search the populated routing table
	n = rt.closest(target)
	require.NotNil(t, n)

	// check all nodes to ensure we actually found the closest node
	var nodes []*node

	for i := range rt.buckets {
		rt.buckets[i].iterate(func(nd *node) {
			nodes = append(nodes, nd)
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		d1 := distance(nodes[i].id, target)
		d2 := distance(nodes[j].id, target)

		// we're sorting for the closest distance,
		// which is actually the greatest number of
		// matching bits, hence why we compare with >
		return d1 > d2
	})

	assert.Equal(t, distance(n.id, target), distance(nodes[0].id, target))
}

func TestRoutingTableFindNearestN(t *testing.T) {
	rt := newRoutingTable(&node{
		id: randomID(),
	})

	// generate a random target key we want to look up
	target := randomID()

	// try to find nodes on an empty table
	ns := rt.closestN(target, 3)
	require.Len(t, ns, 0)

	// insert 10000 nodes into the routing table
	for i := 0; i < 10000; i++ {
		rt.insert(randomID(), nil, time.Duration(0), false)
	}

	// try to find closest nodes on a populated table
	ns = rt.closestN(target, 3)
	require.Len(t, ns, 3)

	// check all nodes to ensure we actually found the closest node
	var nodes []*node

	for i := range rt.buckets {
		rt.buckets[i].iterate(func(nd *node) {
			nodes = append(nodes, nd)
		})
	}

	sort.Slice(nodes, func(i, j int) bool {
		d1 := distance(nodes[i].id, target)
		d2 := distance(nodes[j].id, target)

		// we're sorting for the closest distance,
		// which is actually the greatest number of
		// matching bits, hence why we compare with >
		return d1 > d2
	})

	for i := 0; i < 3; i++ {
		assert.Equal(t, distance(target, nodes[i].id), distance(target, ns[i].id))
	}
}

func BenchmarkRoutingTableFindNearest(b *testing.B) {
	rt := newRoutingTable(&node{
		id: randomID(),
	})

	// insert 10000 nodes into the routing table
	for i := 0; i < 10000; i++ {
		rt.insert(randomID(), nil, time.Duration(0), false)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		target := randomID()

		rt.closest(target)
	}
}

func BenchmarkRoutingTableFindNearestN(b *testing.B) {
	rt := newRoutingTable(&node{
		id: randomID(),
	})

	// insert 10000 nodes into the routing table
	for i := 0; i < 10000; i++ {
		rt.insert(randomID(), nil, time.Duration(0), false)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		target := randomID()

		rt.closestN(target, 3)
	}
}

func BenchmarkRoutingTableInsert(b *testing.B) {
	rt := newRoutingTable(&node{
		id: randomID(),
	})

	nodes := make([][]byte, 10000)

	// preallocate 10,000 nodes
	// should simulate seeing the same
	for i := 0; i < 10000; i++ {
		nodes[i] = randomID()
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rt.insert(nodes[i%10000], nil, time.Duration(0), false)
	}
}

func BenchmarkRoutingTableSeen(b *testing.B) {
	rt := newRoutingTable(&node{
		id: randomID(),
	})

	nodes := make([][]byte, 10000)

	// preallocate 10,000 nodes
	// should simulate seeing the same
	for i := 0; i < 10000; i++ {
		nodes[i] = randomID()
		rt.insert(nodes[i], nil, time.Duration(0), false)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rt.seen(nodes[i%10000])
	}
}
