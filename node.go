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
	"crypto/rand"
	"encoding/binary"
	mrand "math/rand"
	"net"
	"time"
)

func init() {
	s := make([]byte, 8)
	rand.Read(s)
	mrand.Seed(int64(binary.LittleEndian.Uint64(s)))
}

// node represents a node on the network
type node struct {
	// the id of the node (default to 160 bits/20 bytes)
	id []byte
	// the udp address of the node
	address *net.UDPAddr
	// the last time an event was received from this node
	seen time.Time
	// the number of expected responses we are waiting on
	pending int
	// the latency of the node
	latency time.Duration
	// the number of failed attempts to communicate with this node
	failCount int32
	// test mode
	testMode bool
}

func randomID() []byte {
	id := make([]byte, KEY_BYTES)
	rand.Read(id)
	return id
}

func pseudorandomID() []byte {
	id := make([]byte, KEY_BYTES)
	mrand.Read(id)
	return id
}
