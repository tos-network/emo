package main

import (
	"os"

	"github.com/ipfs/emo/cmd/ipfs/emo"
)

func main() {
	os.Exit(emo.Start(emo.BuildDefaultEnv))
}
