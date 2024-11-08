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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPacketManagerFragment(t *testing.T) {
	m := newPacketManager()

	// build a packet that's exactly 3 fragments
	id := randomID()
	data := make([]byte, MaxPayloadSize*3)
	rand.Read(data)

	assert.True(t, m.needsFragmenting(data))

	p := m.fragment(id, data)

	assert.Equal(t, 3, p.frg)
	assert.Equal(t, MaxPacketSize*3, p.len)

	for i := 0; i < 3; i++ {
		pf := p.next()
		require.NotNil(t, data)

		assert.Equal(t, id, pf[:KEY_BYTES])
		assert.Equal(t, byte(i+1), pf[KEY_BYTES])
		assert.Equal(t, byte(3), pf[KEY_BYTES+1])
		assert.Equal(t, data[MaxPayloadSize*i:MaxPayloadSize*(i+1)], pf[PacketHeaderSize:])
	}

	assert.Nil(t, p.next())

	// return the packet so we reuse it
	m.done(p)

	// build a packet that's slightly smaller than 3 max fragments
	id = randomID()
	data = make([]byte, (MaxPayloadSize*3)-300)
	rand.Read(data)

	assert.True(t, m.needsFragmenting(data))

	p = m.fragment(id, data)

	assert.Equal(t, 3, p.frg)
	assert.Equal(t, MaxPacketSize*3-300, p.len)

	read := len(data)

	for i := 0; i < 3; i++ {
		pf := p.next()
		require.NotNil(t, data)

		pread := pf[PacketHeaderSize:]

		assert.Equal(t, id, pf[:KEY_BYTES])
		assert.Equal(t, byte(i+1), pf[KEY_BYTES])
		assert.Equal(t, byte(3), pf[KEY_BYTES+1])

		if read < MaxPayloadSize {
			assert.Equal(t, data[MaxPayloadSize*i:], pread)
		} else {
			assert.Equal(t, data[MaxPayloadSize*i:MaxPayloadSize*(i+1)], pread)
		}

		read = read - len(pread)
	}

	assert.Nil(t, p.next())

	// return the packet so we reuse it
	m.done(p)
}

func TestPacketManagerAssemble(t *testing.T) {
	m := newPacketManager()

	id := randomID()
	data := make([]byte, MaxPayloadSize*5)
	rand.Read(data)

	p := m.fragment(id, data)

	var fragments [][]byte

	f := p.next()

	for f != nil {
		fragments = append(fragments, f)
		f = p.next()
	}

	// assemble them "out of order" (in this case, just reverse order)
	for i := p.frg - 1; i > 0; i-- {
		p := m.assemble(fragments[i])
		assert.Nil(t, p)
	}

	// on the last fragment, we should be returned a full packet
	p = m.assemble(fragments[0])
	assert.NotNil(t, p)
	assert.Equal(t, data, p.data())

	m.done(p)
}

func TestPacketManagerFragmentAssemble(t *testing.T) {
	m := newPacketManager()

	id := randomID()
	data := make([]byte, MaxPayloadSize/2)
	rand.Read(data)

	p := m.fragment(id, data)

	f := p.next()

	assert.Len(t, f, len(data)+PacketHeaderSize)

	// on the last fragment, we should be returned a full packet
	p = m.assemble(f)
	assert.NotNil(t, p)
	assert.Equal(t, data, p.data())

	m.done(p)

	id = randomID()
	data = make([]byte, 38424)
	rand.Read(data)

	p = m.fragment(id, data)

	var fragments [][]byte

	f = p.next()

	for f != nil {
		fragments = append(fragments, f)
		f = p.next()
	}

	var p2 *packet

	for i := range fragments {
		p2 = m.assemble(fragments[i])
	}

	assert.Equal(t, data, p2.data())

	m.done(p)
}
