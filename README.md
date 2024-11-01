<h1 align="center">
  <br>
  <br>
  Emo: IPFS Implementation in GO
  <br>
</h1>

<hr />

## What is Emo?

Emo is an IPFS implementation, built on the TOS Network, Emo is a distributed data storage and retrieval system with a lot of interconnected nodes..


### Build from Source

Download and Compile Emo

```
$ git clone https://github.com/tos-network/emo.git

$ cd emo
$ make install
```

## Getting Started

### Usage

To start using IPFS, you must first initialize IPFS's config files on your
system, this is done with `ipfs init`. See `ipfs init --help` for information on
the optional arguments it takes. After initialization is complete, you can use
`ipfs mount`, `ipfs add` and any of the other commands to explore!

### Some things to try

Basic proof of 'ipfs working' locally:

    echo "hello world" > hello
    ipfs add hello
    # This should output a hash string that looks something like:
    # QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
    ipfs cat <that hash>


### Testing

```
make test
```

## License

This project is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/tos-network/emo/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/tos-network/emo/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)
