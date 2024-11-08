# Emo 

Emo is a Kademlia DHT implementation for Go with a focus on performance and ease of use. It is not seek to conform to any existing standards or implementations. 

 - implements a 256 bit keyspace using Keccak256 hashing.
 - replication factor (K) of 20
 - wire protocol using flatbuffers
 - SO_REUSEPORT to concurrently handle requests on the same port
 - asynchronous api
 - supports values larger than MTU

## Usage

In order to start a cluster of nodes, you will first need a bootstrap node that all other nodes can connect to first. To start a bootstrap node:

```go
func main() {
    cfg := &dht.Config{
        ListenAddress: "127.0.0.1:9000", // udp address to bind to
        Listeners: 4,                    // number of socket listeners, defaults to GOMAXPROCS
        Timeout: time.Minute / 2         // request timeout, defaults to 1 minute
    }

    dht, err := dht.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    log.Println("bootstrap node started!")
}
```

Once a bootstrap node is up and runing, you can add other nodes to the network:

```go
func main() {
    cfg := &dht.Config{
        ListenAddress: "127.0.0.1:9001", // udp address to bind to
        BootstrapAddresses: []string{
            "127.0.0.1:9000",
        },
        Listeners: 4,                    // number of socket listeners, defaults to GOMAXPROCS
        Timeout: time.Minute / 2         // request timeout, defaults to 1 minute
    }

    dht, err := dht.New(cfg)
    if err != nil {
        log.Fatal(err)
    }

    log.Println("node started!")
}
```

From any node you can then store values as follows:
```go
func main() {
    ...

    // helper function to construct a sha1 hash that
    // will be used as the values key
    myKey := dht.Key("my-awesome-key")
    myValue := []byte("my-even-more-awesome-value")

    // stores a value for a given amount of time
    dht.Store(myKey, myValue, time.Hour, func(err error) {
        if err != nil {
            log.Fatal(err)
        }
        log.Printf("successfully stored key: %s -> %s", string(myKey), string(myValue))
    })
}
```

Once your value is stored, you can retreive it from the network as follows:
```go
func main() {
    ...

    // finds the value. please note it is not safe to use the value outside
    // of the provided callback unless it is copied
    dht.Find(myKey, func(value []byte, err error) {
        if err != nil {
            log.Fatal(err)
        }
        log.Printf("successfully retrieved key: %s -> %s !\n", string(myKey), string(value))
    })
}
```

## OS Tuning

For most linux distros, socket send and receive buffers are set very low. This will almost certainly result in large amounts of packet loss at higher throughput levels as these buffers get overrun.

How large these buffers will need to be will be dependent on your workload, so you should experiment to find the correct value.

You can temporarily increase the of the read and write buffers via sysctl:
```sh
# set the rmem and wmem buffers to ~128 MB each
$ sysctl -w net.core.rmem_max=536870912 && sysctl -w net.core.rmem_default=134217728 && sysctl -w net.core.wmem_max=536870912 && sysctl -w net.core.wmem_default=134217728
```

## Development

To re-generate the flatbuffer definitions for the wire protocol:
```sh
$ make generate
```

Build the CLI emo executable:
```sh
$ go build -o emo ./cmd/emo
```
This will generate an executable named emo.

To run tests:
```sh
$ go test -v -race
```

To run benchmarks:
```sh
$ go test -v -bench=.
```
Running the Daemon
To run the emo daemon as a background process (similar to a service), you can use the following methods:
```sh
$ ./emo daemon
```
This will start the emo daemon listening on 0.0.0.0:9000.

Running the Daemon in the Background
```sh
$ nohup ./emo daemon > emo.log 2>&1 &
```
This will start the emo daemon in the background and output the log to emo.log.

## Implemented
- [✅] routing
- [✅] storage (custom)
- [✅] storage (in-memory)
- [✅] ping
- [✅] store
- [✅] findNode
- [✅] findValue
- [✅] benchmarks
- [✅] node join/leave
- [✅] user defined storage
- [✅] multiple values per store request
- [✅] handles packets larger than MTU
- [✅] multiple values per key
- [✅] batch socket reads and writes
- [✅] peer refresh
- [✅] key refresh
- [✅] latency based route selection

## Future Improvements
- [ ] io_uring socket handler
- [ ] storage (persistent)
- [ ] NAT traversal
- [ ] support SO_REUSEPORT on mac/windows
- [ ] configurable logging
- [ ] ntp time