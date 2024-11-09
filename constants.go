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

const (
	// K number of nodes in a bucket
	K = 20
	// ALPHA number of nodes to query in parallel
	ALPHA = 3
	// KEY_BITS number of bits in a key
	KEY_BITS = 256
	// KEY_BYTES number of bytes in a key
	KEY_BYTES = KEY_BITS / 8
	// VALUE_BYTES maximum bytecode size that a value can have.
	VALUE_BYTES = 32 * 1024 // 32KiB
)
