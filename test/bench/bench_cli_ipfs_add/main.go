package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/ipfs/emo/thirdparty/unit"

	random "github.com/ipfs/go-test/random"
	config "github.com/ipfs/emo/config"
)

var (
	debug  = flag.Bool("debug", false, "direct ipfs output to console")
	online = flag.Bool("online", false, "run the benchmarks with a running daemon")
)

func main() {
	flag.Parse()
	if err := compareResults(); err != nil {
		log.Fatal(err)
	}
}

func compareResults() error {
	var amount unit.Information
	for amount = 10 * unit.MB; amount > 0; amount = amount * 2 {
		if results, err := benchmarkAdd(int64(amount)); err != nil { // TODO compare
			return err
		} else {
			log.Println(amount, "\t", results)
		}
	}
	return nil
}

func benchmarkAdd(amount int64) (*testing.BenchmarkResult, error) {
	var benchmarkError error
	results := testing.Benchmark(func(b *testing.B) {
		b.SetBytes(amount)
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			tmpDir := b.TempDir()

			env := append(
				[]string{fmt.Sprintf("%s=%s", config.EnvDir, path.Join(tmpDir, config.DefaultPathName))}, // first in order to override
				os.Environ()...,
			)
			setupCmd := func(cmd *exec.Cmd) {
				cmd.Env = env
				if *debug {
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
				}
			}

			initCmd := exec.Command("ipfs", "init")
			setupCmd(initCmd)
			if err := initCmd.Run(); err != nil {
				benchmarkError = err
				b.Fatal(err)
			}

			const seed = 1
			f, err := os.CreateTemp("", "")
			if err != nil {
				benchmarkError = err
				b.Fatal(err)
			}
			defer os.Remove(f.Name())

			randReader := &io.LimitedReader{
				R: random.NewSeededRand(seed),
				N: amount,
			}
			if _, err := io.Copy(f, randReader); err != nil {
				benchmarkError = err
				b.Fatal(err)
			}
			if err := f.Close(); err != nil {
				benchmarkError = err
				b.Fatal(err)
			}

			func() {
				// FIXME online mode isn't working. client complains that it cannot open leveldb
				if *online {
					daemonCmd := exec.Command("ipfs", "daemon")
					setupCmd(daemonCmd)
					if err := daemonCmd.Start(); err != nil {
						benchmarkError = err
						b.Fatal(err)
					}
					defer func() {
						_ = daemonCmd.Process.Signal(os.Interrupt)
						_ = daemonCmd.Wait()
					}()
				}

				b.StartTimer()
				addCmd := exec.Command("ipfs", "add", f.Name())
				setupCmd(addCmd)
				if err := addCmd.Run(); err != nil {
					benchmarkError = err
					b.Fatal(err)
				}
				b.StopTimer()
			}()
		}
	})
	if benchmarkError != nil {
		return nil, benchmarkError
	}
	return &results, nil
}
