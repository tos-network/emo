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
	"errors"
	"hash/maphash"
	"sync"
	"time"

	"github.com/tos-network/emo/protocol"
)

var (
	// ErrRequestTimeout returned when a pending request has not recevied a response before the TTL period
	ErrRequestTimeout = errors.New("request timeout")
)

// a pending request
type request struct {
	callback func(event *protocol.Event, err error) bool
	ttl      time.Time
}

// cache tracks asynchronous event requests
type cache struct {
	requests sync.Map
	hasher   sync.Pool
}

func newCache(refresh time.Duration) *cache {
	seed := maphash.MakeSeed()

	c := &cache{
		hasher: sync.Pool{
			New: func() any {
				var hasher maphash.Hash
				hasher.SetSeed(seed)
				return &hasher
			},
		},
	}

	go c.cleanup(refresh)

	return c
}

func (c *cache) set(key []byte, ttl time.Time, cb func(*protocol.Event, error) bool) {
	r := &request{callback: cb, ttl: ttl}

	h := c.hasher.Get().(*maphash.Hash)

	h.Reset()
	h.Write(key)

	k := h.Sum64()

	c.hasher.Put(h)

	c.requests.Store(k, r)
}

func (c *cache) callback(key []byte, event *protocol.Event, err error) {
	h := c.hasher.Get().(*maphash.Hash)

	h.Reset()
	h.Write(key)

	k := h.Sum64()

	c.hasher.Put(h)

	r, ok := c.requests.Load(k)
	if !ok {
		return
	}

	if r.(*request).callback(event, err) {
		c.requests.Delete(k)
	}
}

func (c *cache) cleanup(refresh time.Duration) {
	for {
		time.Sleep(refresh)

		now := time.Now()

		c.requests.Range(func(key, value any) bool {
			v := value.(*request)

			if now.After(v.ttl) {
				v.callback(nil, ErrRequestTimeout)
				c.requests.Delete(key)
			}

			return true
		})
	}
}
