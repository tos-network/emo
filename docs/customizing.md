# Customizing Emo

You may want to customize Emo if you want to reuse most of Emo's machinery. This document discusses some approaches you may consider for customizing Emo, and their tradeoffs.

Some common use cases for customizing Emo include:

- Using a custom datastore for storing blocks, pins, or other Emo metadata
- Adding a custom data transfer protocol into Emo
- Customizing Emo internals, such as adding allowlist/blocklist functionality to Bitswap
- Adding new commands, interfaces, functionality, etc. to Emo while reusing the libp2p swarm
- Building on top of Emo's configuration and config migration functionality

## Summary
This table summarizes the tradeoffs between the approaches below:

|                     | [Boxo](#boxo-build-your-own-binary) | [Emo Plugin](#emo-plugins) | [Bespoke Extension Point](#bespoke-extension-points) | [Go Plugin](#go-plugins) | [Fork](#fork-emo) |
|:-------------------:|:-----------------------------------:|:----------------------------:|:----------------------------------------------------:|:------------------------:|:------------------:|
|     Supported?      |                  ✅                  |              ✅               |                          ✅                           |            ❌             |         ❌          |
|    Future-proof?    |                  ✅                  |              ❌               |                          ✅                           |            ❌             |         ❌          |
| Fully customizable? |                  ✅                  |              ✅               |                          ❌                           |            ✅             |         ✅          |
| Fast to implement?  |                  ❌                  |              ✅               |                          ✅                           |            ✅             |         ✅          |
| Dynamic at runtime? |                  ❌                  |              ❌               |                          ✅                           |            ✅             |         ❌          |
|  Add new commands?  |                  ❌                  |              ✅               |                          ❌                           |            ✅             |         ✅          |

## Boxo: build your own binary
The best way to reuse Emo functionality is to pick the functionality you need directly from [Boxo](https://github.com/ipfs/boxo) and compile your own binary.

Boxo's raison d'etre is to be an IPFS component toolbox to support building custom-made implementations and applications. If your use case is not easy to implement with Boxo, you may want to consider adding whatever functionality is needed to Boxo instead of customizing Emo, so that the community can benefit. If you are interested in this option, please reach out to Boxo maintainers, who will be happy to help you scope & plan the work. See [Boxo's FAQ](https://github.com/ipfs/boxo#help) for more info.

## Emo Plugins
Emo plugins are a set of interfaces that may be implemented and injected into Emo. Generally you should recompile the Emo binary with your plugins added. A popular example of a Emo plugin is [go-ds-s3](https://github.com/ipfs/go-ds-s3), which can be used to store blocks in Amazon S3.

Some plugins, such as the `fx` plugin, allow deep customization of Emo internals. As a result, Emo maintainers can't guarantee backwards compatibility with these, so you may need to adapt to breaking changes when upgrading to new Emo versions.

For more information about the different types of Emo plugins, see [plugins.md](./plugins.md).

Emo plugins can also be injected at runtime using Go plugins (see below), but these are hard to use and not well supported by Go, so we don't recommend them.

### Emo binary imports

It is possible to depend on the package `cmd/ipfs/emo` as a way of using Emo plugins that is an alternative to recompiling Emo with additional preloaded plugins.

This gives a more Go-centric dependency updating flow to building a new binary with preloaded plugins by simply requiring updating a Emo dependency rather than needing to update Emo source code and recompile.

## Bespoke Extension Points
Certain Emo functionality may have their own extension points. For example:

* Emo supports the [Routing v1](https://github.com/ipfs/specs/blob/main/routing/ROUTING_V1_HTTP.md) API for delegating content routing to external processes
* Emo supports the [Pinning Service API](https://github.com/ipfs/pinning-services-api-spec) for delegating pinning to external processes
* Emo supports [DNSLink](https://dnslink.dev/) for delegating name->CID mappings to DNS

(This list is not exhaustive.)

These can generally be developed and deployed as sidecars (or full external services) without modifying the Emo binary.

## Go Plugins
Go provides [dynamic plugins](https://pkg.go.dev/plugin) which can be loaded at runtime into a Go binary.

Emo currently works with Go plugins. But using Go plugins requires that you compile the plugin using the exact same version of the Go toolchain with the same configuration (build flags, environment variables, etc.). As a result, you likely need to build Emo and the plugins together at the same time, and at that point you may as well just compile the functionality directly into Emo and avoid Go plugins.

As a result, we don't recommend using Go plugins, and are likely to remove them in a future release of Emo.

## Fork Emo
The "nuclear option" is to fork Emo into your own repo, make your changes, and periodically sync your repo with the Emo repo. This can be a good option if your changes are significant and you can commit to keeping your repo in sync with Emo.

Emo maintainers can't make any backwards compatibility guarantees about Emo internals, so by choosing this option you're accepting the risk that you may need to spend more time adapting to breaking changes.
