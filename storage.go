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
	"hash/maphash"
	"log"
	"sync"
	"time"
)

// StorageType defines the type of storage to use.
type StorageType string

const (
	InMemoryStorage StorageType = "inmemory"
	LevelDBStorage  StorageType = "leveldb"
)

// InitializeStorage initializes the storage based on the configuration.
func InitializeStorage(cfg *Config) (Storage, error) {
	switch cfg.StorageBackend {
	case InMemoryStorage:
		return newInMemoryStorage(), nil
	case LevelDBStorage:
		log.Println("Using LevelDB storage")
		if cfg.LevelDBPath == "" {
			if cfg.DataDir == "" {
				cfg.DataDir = DefaultDataDir()
			}
			cfg.LevelDBPath = ChaindataDir(cfg.DataDir)
		}
		log.Printf("Using LevelDB storage at %s\n", cfg.LevelDBPath)
		return NewDatabase(cfg.LevelDBPath)
	default:
		return newInMemoryStorage(), nil
	}
}

// Storage defines the storage interface used by the DLT
type Storage interface {
	Get(key []byte, from time.Time) ([]*Value, bool)
	Set(key, value []byte, created time.Time, ttl time.Duration) bool
	Iterate(cb func(value *Value) bool)
}

// Value represents the value to be stored
type Value struct {
	Key     []byte
	Value   []byte
	TTL     time.Duration
	Created time.Time
	expires time.Time
}

type item struct {
	contains map[uint64]struct{}
	values   []*Value
	mu       sync.Mutex
}

func (i *item) insert(hash uint64, value *Value) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	_, ok := i.contains[hash]
	if ok {
		return true
	}

	// TODO this will be really slow, but good enough for now
	i.contains[hash] = struct{}{}
	i.values = append(i.values, value)

	return true
}

// implement simple storage for now storage
type storage struct {
	store  sync.Map
	hasher sync.Pool
}

func newInMemoryStorage() *storage {
	// TODO : this will probably cause collisions
	// that need to be handled!
	seed := maphash.MakeSeed()

	s := &storage{
		store: sync.Map{},
		hasher: sync.Pool{
			New: func() any {
				var hasher maphash.Hash
				hasher.SetSeed(seed)
				return &hasher
			},
		},
	}

	go s.cleanup()

	return s
}

// Get gets a key by its id
func (s *storage) Get(k []byte, from time.Time) ([]*Value, bool) {
	h := s.hasher.Get().(*maphash.Hash)

	h.Reset()
	h.Write(k)
	key := h.Sum64()

	s.hasher.Put(h)

	v, ok := s.store.Load(key)
	if !ok {
		return nil, false
	}

	it := v.(*item)

	// if we don't need to filter the query, then return all values
	if from.IsZero() {
		return v.(*item).values, true
	}

	var index int

	// filter the query to values after a given date
	for i := 0; i < len(it.values); i++ {
		if it.values[i].Created.Before(from) {
			// TODO : might be a bit wonky if created from timestamps are not in order
			index++
		}
	}

	// we have no results left that are valid for the query
	if index >= len(it.values) {
		return nil, false
	}

	// TODO : actually store and return multiple values
	return it.values[index:], true
}

// Set sets a key value pair for a given ttl
func (s *storage) Set(k, v []byte, created time.Time, ttl time.Duration) bool {
	// we keep a copy of the key and value as it's actually
	// read from a buffer that's going to be reused
	// so we need to store this as a copy to avoid
	// it getting overwritten by other data

	kc := make([]byte, len(k))
	copy(kc, k)

	vc := make([]byte, len(v))
	copy(vc, v)

	h := s.hasher.Get().(*maphash.Hash)

	h.Reset()
	h.Write(k)
	key := h.Sum64()

	// hash the value so we can check if we have stored it already
	h.Reset()
	h.Write(v)
	vh := h.Sum64()

	s.hasher.Put(h)

	value := &Value{
		Key:     kc,
		Value:   vc,
		TTL:     ttl,
		Created: created,
		expires: time.Now().Add(ttl),
	}

	// loading first is apparently faster?
	actual, ok := s.store.Load(key)
	if ok {
		return actual.(*item).insert(vh, value)
	}

	actual, ok = s.store.LoadOrStore(key, &item{
		contains: map[uint64]struct{}{vh: {}},
		values:   []*Value{value},
	})

	if !ok {
		return true
	}

	return actual.(*item).insert(vh, value)
}

// Iterate iterates over keys in the storage
func (s *storage) Iterate(cb func(v *Value) bool) {
	s.store.Range(func(ky any, vl any) bool {
		item := vl.(*item)

		item.mu.Lock()

		for i := range item.values {
			if !cb(item.values[i]) {
				return false
			}
		}

		item.mu.Unlock()

		return true
	})
}

func (s *storage) cleanup() {
	for {
		// scan the storage to check for values that have expired
		time.Sleep(time.Minute)

		now := time.Now()

		s.store.Range(func(ky any, vl any) bool {
			item := vl.(*item)
			item.mu.Lock()

			for i := range item.values {
				if item.values[i].expires.After(now) {
					s.store.Delete(ky)
				}
			}

			item.mu.Unlock()

			return true
		})
	}
}
