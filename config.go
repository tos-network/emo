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

import "time"

// Config configuration parameters for the dht
type Config struct {
	// LocalID the id of this node. If not specified, a random id will be generated
	LocalID []byte
	// ListenAddress the udp ip and port to listen on
	ListenAddress string
	// BootstrapAddresses the udp ip and port of the bootstrap nodes
	BootstrapAddresses []string
	// Listeners the number of threads that will listen on the designated udp port
	Listeners int
	// Timeout the amount of time before a peer is declared unresponsive and removed
	Timeout time.Duration
	// Storage implementation to use for storing key value pairs
	Storage Storage
	// StorageBackend the type of storage to use
	StorageBackend StorageType
	// LevelDBPath the path to the LevelDB database
	LevelDBPath string
	// DataDir the path to the data directory
	DataDir string
	// SocketBufferSize sets the size of the udp sockets send and receive buffer
	SocketBufferSize int
	// SocketBatchSize the batch size of udp messages that will be written to the underlying socket
	SocketBatchSize int
	// SocketBatchInterval the period with which the current batch of udp messages will be written to the underlying socket if not full
	SocketBatchInterval time.Duration
	// Logging enables basic logging
	Logging bool
}
