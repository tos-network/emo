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
	"os"
	"path/filepath"
	"runtime"
)

// DefaultDataDir returns the default data directory path based on the operating system.
func DefaultDataDir() string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(os.Getenv("HOME"), "Library", "Tos")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Tos")
	default: // unix-like
		return filepath.Join(os.Getenv("HOME"), ".tos")
	}
}

// ChaindataDir returns the path to the LevelDB database.
func ChaindataDir(dataDir string) string {
	return filepath.Join(dataDir, "gtos", "chaindata")
}
