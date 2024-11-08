package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/tos-network/emo"
)

func main() {
	daemonCmd := flag.NewFlagSet("daemon", flag.ExitOnError)
	listenAddress := daemonCmd.String("listen", "0.0.0.0:9000", "address to listen on")
	listeners := daemonCmd.Int("listeners", 4, "number of socket listeners")
	timeout := daemonCmd.Duration("timeout", time.Minute/2, "request timeout")

	if len(os.Args) < 2 {
		fmt.Println("expected 'daemon' subcommand")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "daemon":
		daemonCmd.Parse(os.Args[2:])

		cfg := &emo.Config{
			ListenAddress:  *listenAddress,
			Listeners:      *listeners,
			Timeout:        *timeout,
			StorageBackend: emo.LevelDBStorage,
		}

		dht, err := emo.New(cfg)
		if err != nil {
			log.Fatalf("failed to start emo daemon: %v", err)
		}

		log.Printf("emo daemon started on %s\n", *listenAddress)

		// Handle shutdown signals
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c

		log.Println("emo daemon shutting down...")
		dht.Close()
		log.Println("emo daemon stopped.")
	default:
		fmt.Println("expected 'daemon' subcommand")
		os.Exit(1)
	}
}
