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

// FindOption for configuring find requests
type FindOption struct {
	from time.Time
}

// ValuesFrom filters results to only those that were created after a given timestmap
// this is useful for repeat queries where duplicates ideally should be avoided
func ValuesFrom(from time.Time) *FindOption {
	return &FindOption{
		from: from,
	}
}
