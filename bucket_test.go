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
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBucketInsertAndGet(t *testing.T) {
	b := bucket{
		nodes:  make([]*node, 20),
		expiry: time.Minute,
	}

	ids := make([][]byte, 100)

	for i := 0; i < 100; i++ {
		id := make([]byte, 32)
		rand.Read(id)

		ids[i] = id
		b.insert(id, nil, time.Duration(0), false)
	}

	assert.Equal(t, 20, b.size)

	for i := range ids {
		if i >= 20 {
			var found bool

			for x := 0; x < b.size; x++ {
				if bytes.Equal(b.nodes[x].id, ids[i]) {
					found = true
					break
				}
			}

			assert.False(t, found)
		}
	}
}
