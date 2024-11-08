package emo

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"log"
	"sync"
	"time"

	"hash/maphash"

	"github.com/syndtr/goleveldb/leveldb"
)

// database implements the Storage interface using LevelDB.
type database struct {
	db     *leveldb.DB
	hasher sync.Pool
}

// Newdatabase initializes a new database instance.
func NewDatabase(path string) (*database, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}

	seed := maphash.MakeSeed()

	storage := &database{
		db: db,
		hasher: sync.Pool{
			New: func() any {
				var hasher maphash.Hash
				hasher.SetSeed(seed)
				return &hasher
			},
		},
	}

	// Start a background goroutine to clean up expired entries.
	go storage.cleanup()

	return storage, nil
}

// Get retrieves values associated with the given key.
// If 'from' is not zero, it filters out values created before the 'from' timestamp.
func (s *database) Get(k []byte, from time.Time) ([]*Value, bool) {
	h := s.hasher.Get().(*maphash.Hash)
	h.Reset()
	h.Write(k)
	key := h.Sum64()
	s.hasher.Put(h)

	keyBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(keyBytes, key)

	data, err := s.db.Get(keyBytes, nil)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return nil, false
		}
		return nil, false
	}

	// Deserialize the stored values.
	// Assuming values are stored in binary format. Adjust as needed.
	var values []*Value
	err = deserializeValues(data, &values)
	if err != nil {
		return nil, false
	}

	if from.IsZero() {
		return values, true
	}

	var filtered []*Value
	for _, v := range values {
		if v.Created.After(from) || v.Created.Equal(from) {
			filtered = append(filtered, v)
		}
	}

	if len(filtered) == 0 {
		return nil, false
	}

	return filtered, true
}

// Set stores a key-value pair with a specified TTL.
func (s *database) Set(k, v []byte, created time.Time, ttl time.Duration) bool {
	// Create copies of key and value to ensure immutability.
	kc := make([]byte, len(k))
	copy(kc, k)

	vc := make([]byte, len(v))
	copy(vc, v)

	h := s.hasher.Get().(*maphash.Hash)
	h.Reset()
	h.Write(k)
	key := h.Sum64()
	s.hasher.Put(h)

	keyBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(keyBytes, key)

	value := &Value{
		Key:     kc,
		Value:   vc,
		TTL:     ttl,
		Created: created,
		expires: time.Now().Add(ttl),
	}

	// Serialize the value.
	data, err := serializeValue(value)
	if err != nil {
		return false
	}

	// Atomically update the value in LevelDB.
	// Using a write batch for atomicity.
	batch := new(leveldb.Batch)
	batch.Put(keyBytes, data)

	err = s.db.Write(batch, nil)
	if err != nil {
		return false
	}

	return true
}

// Iterate iterates over all stored values and applies the callback.
// If the callback returns false, iteration stops.
func (s *database) Iterate(cb func(v *Value) bool) {
	iter := s.db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		data := iter.Value()
		var value Value
		err := deserializeValues(data, &[]*Value{&value})
		if err != nil {
			continue
		}

		if !cb(&value) {
			break
		}
	}

	if err := iter.Error(); err != nil {
		log.Println("LevelDB Iteration Error:", err)
	}
}

// Close closes the LevelDB database.
func (s *database) Close() error {
	return s.db.Close()
}

// cleanup periodically removes expired entries from the database.
func (s *database) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			iter := s.db.NewIterator(nil, nil)
			for iter.Next() {
				keyBytes := iter.Key()
				data := iter.Value()

				var values []*Value
				err := deserializeValues(data, &values)
				if err != nil {
					continue
				}

				var valid []*Value
				for _, v := range values {
					if v.expires.After(now) {
						valid = append(valid, v)
					}
				}

				if len(valid) == 0 {
					// Delete the key if no valid values remain.
					s.db.Delete(keyBytes, nil)
					continue
				}

				// Serialize the remaining valid values.
				newData, err := serializeValues(valid)
				if err != nil {
					continue
				}

				// Update the entry with valid values.
				s.db.Put(keyBytes, newData, nil)
			}
			iter.Release()
			if err := iter.Error(); err != nil {
				log.Println("LevelDB Cleanup Iteration Error:", err)
			}
		}
	}
}

// serializeValue serializes a single Value into a byte slice.
// Implement this function based on your serialization format.
func serializeValue(v *Value) ([]byte, error) {
	// Example using encoding/gob
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// serializeValues serializes multiple Values into a single byte slice.
// Implement this function based on your serialization format.
func serializeValues(values []*Value) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(values)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// deserializeValues deserializes a byte slice into a slice of Values.
// Implement this function based on your serialization format.
func deserializeValues(data []byte, values *[]*Value) error {
	// Example using encoding/gob
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(values)
}
