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
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJourneyAddRoutes(t *testing.T) {
	target := randomID()

	// test that nodes get added
	j := newJourney(randomID(), target, 5)

	nodes := []*node{
		{id: randomID()},
		{id: randomID()},
		{id: randomID()},
		{id: randomID()},
		{id: randomID()},
	}

	j.add(nodes)

	assert.Equal(t, target, j.destination)
	assert.Equal(t, 5, j.remaining)
	assert.Equal(t, 5, j.routes)

	for i := 0; i < 5; i++ {
		assert.NotNil(t, j.nodes[i])
		assert.Greater(t, KEY_BITS, j.distances[i])
	}

	// test that we evict routes that are not
	// closer than the new node we provide
	j = newJourney(randomID(), target, 5)

	// insert the maximum amount of nodes
	nodes = make([]*node, K)

	for i := 0; i < K; i++ {
		nodes[i] = &node{
			id: randomID(),
		}
	}

	j.add(nodes)

	// generate a node that will be a better match
	var betterRoute *node
	var betterDistance int

	for {
		id := randomID()
		d := distance(id, target)
		if d > j.distances[0] {
			betterRoute = &node{id: id}
			betterDistance = d
			break
		}
	}

	// add the new route, which should evict the worst one
	j.add([]*node{betterRoute})

	assert.Equal(t, betterRoute.id, j.nodes[0].id)
	assert.Equal(t, betterDistance, j.distances[0])
}

func TestJourneyNextRoutes(t *testing.T) {
	target := randomID()

	// test that nodes are removed correctly
	j := newJourney(randomID(), target, 5)

	assert.Nil(t, j.next(5))

	// insert the maximum amount of nodes
	nodes := make([]*node, K)

	for i := 0; i < K; i++ {
		addr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.01:%d", 9000+i))

		nodes[i] = &node{
			id:      randomID(),
			address: addr,
		}
	}

	j.add(nodes)

	for i := 0; i < 4; i++ {
		next := j.next(K / 4)
		assert.Len(t, next, K/4)
		assert.Equal(t, K-((K/4)*(i+1)), j.routes)
		// TODO : check returned are best n routes
	}

	// test that maximum iteration limit
	j = newJourney(randomID(), target, 1)

	// insert the maximum amount of nodes
	nodes = make([]*node, K)

	for i := 0; i < K; i++ {
		nodes[i] = &node{
			id: randomID(),
		}
	}

	j.add(nodes)

	assert.NotNil(t, j.next(5))
	assert.Nil(t, j.next(5))
}

func BenchmarkJourneyAddRoutes(b *testing.B) {
	target := randomID()

	j := newJourney(randomID(), target, 5)

	nodes := make([][]*node, 10000)

	for i := 0; i < 10000; i++ {
		nodes[i] = []*node{
			{id: randomID()},
			{id: randomID()},
			{id: randomID()},
			{id: randomID()},
			{id: randomID()},
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		j.add(nodes[i%1000])
	}
}
