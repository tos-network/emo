# Plugins

Since 0.4.11 Emo has an experimental plugin system that allows augmenting
the daemons functionality without recompiling.

When an IPFS node is started, it will load plugins from the `$IPFS_PATH/plugins`
directory (by default `~/.ipfs/plugins`).

**Table of Contents**

- [Plugin Types](#plugin-types)
    - [IPLD](#ipld)
    - [Datastore](#datastore)
- [Available Plugins](#available-plugins)
- [Installing Plugins](#installing-plugins)
    - [External Plugin](#external-plugin)
        - [In-tree](#in-tree)
        - [Out-of-tree](#out-of-tree)
    - [Preloaded Plugins](#preloaded-plugins)
- [Creating A Plugin](#creating-a-plugin)

## Plugin Types

Plugins can implement one or more plugin types, defined in the
[plugin](https://godoc.org/github.com/ipfs/emo/plugin) package.

### IPLD

IPLD plugins add support for additional formats to `ipfs dag` and other IPLD
related commands.

### Datastore

Datastore plugins add support for additional datastore backends.

### Tracer

(experimental)

Tracer plugins allow injecting an opentracing backend into Emo.

### Daemon

Daemon plugins are started when the Emo daemon is started and are given an
instance of the CoreAPI. This should make it possible to build an ipfs-based
application without IPC and without forking Emo.

Note: We eventually plan to make Emo usable as a library. However, this
plugin type is likely the best interim solution.

### fx (experimental)

Fx plugins let you customize the [fx](https://pkg.go.dev/go.uber.org/fx) dependency graph and configuration,
by customizing the`fx.Option`s that are passed to `fx` when the Emo node is initialized.

For example, you can override an interface such as [exchange.Interface](https://github.com/ipfs/go-ipfs-exchange-interface)
or [pin.Pinner](https://github.com/ipfs/go-ipfs-pinner) with a custom implementation by appending an option like
`fx.Decorate(func() exchange.Interface { return customExchange })`.

Fx supports some advanced customization. Simple interface replacements like above are unlikely to break in the future, 
but the more invasive your changes, the more likely they are to break between releases. Emo cannot guarantee backwards
compatibility for `fx` customizations.

Fx options are applied across every execution of the `ipfs` binary, including:

- Repo initialization
- Daemon
- Applying migrations
- etc.

So if you plug in a blockservice that disallows non-allowlisted CIDs, then this may break migrations
that fetch migration code over the IPFS network.

### Internal

(never stable)

Internal plugins are like daemon plugins _except_ that they can access, replace,
and modify all internal state. Use this plugin type to extend Emo in
arbitrary ways. However, be aware that your plugin will likely break every time
Emo updated.

## Configuration

Plugins can be configured in the `Plugins` section of the config file. Here,
plugins can be:

1. Passed an arbitrary config object via the `Config` field.
2. Disabled via the `Disabled` field.

Example:

```js
{
  // ...
  "Plugins": {
    "Plugins": {
      // plugin named "plugin-foo"
      "plugin-foo": {
        "Config": { /* arbitrary json */ }
      },
      // plugin named "plugin-bar"
      "plugin-bar": {
        "Disabled": true // "plugin-bar" will not be loaded
      }
    }
  }
}
```

## Available Plugins

| Name                                                                            | Type      | Preloaded | Description                                    |
|---------------------------------------------------------------------------------|-----------|-----------|------------------------------------------------|
| [git](https://github.com/ipfs/emo/tree/master/plugin/plugins/git)           | IPLD      | x         | An IPLD format for git objects.                |
| [badgerds](https://github.com/ipfs/emo/tree/master/plugin/plugins/badgerds) | Datastore | x         | A high performance but experimental datastore. |
| [flatfs](https://github.com/ipfs/emo/tree/master/plugin/plugins/flatfs)     | Datastore | x         | A stable filesystem-based datastore.           |
| [levelds](https://github.com/ipfs/emo/tree/master/plugin/plugins/levelds)   | Datastore | x         | A stable, flexible datastore backend.          |
| [jaeger](https://github.com/ipfs/go-jaeger-plugin)                              | Tracing   |           | An opentracing backend.                        |

* **Preloaded** plugins are built into the Emo binary and do not need to be
  installed separately. At the moment, all in-tree plugins are preloaded.

## Installing Plugins

Emo supports two types of plugins: External and Preloaded.

* External plugins must be installed in `$IPFS_PATH/plugins/` (usually
`~/.ipfs/plugins/`).
* Preloaded plugins are built-into the Emo when it's compiled.

### External Plugin

The advantage of an external plugin is that it can be built, packaged, and
installed independently of Emo. Unfortunately, this method is only supported
on Linux and MacOS at the moment. Users of other operating systems should follow
the instructions for preloaded plugins.

#### In-tree

To build plugins included in
[plugin/plugins](https://github.com/ipfs/emo/tree/master/plugin/plugins),
run:

```bash
emo$ make build_plugins
emo$ ls plugin/plugins/*.so
```

To install, copy desired plugins to `$IPFS_PATH/plugins`. For example:

```bash
emo$ mkdir -p ~/.ipfs/plugins/
emo$ cp plugin/plugins/git.so ~/.ipfs/plugins/
emo$ chmod +x ~/.ipfs/plugins/git.so # ensure plugin is executable
```

Finally, restart daemon if it is running.

#### Out-of-tree

To build out-of-tree plugins, use the plugin's Makefile if provided. Otherwise,
you can manually build the plugin by running:

```bash
myplugin$ go build -buildmode=plugin -o myplugin.so myplugin.go
```

Finally, as with in-tree plugins:

1. Install the plugin in `$IPFS_PATH/plugins`.
2. Mark the plugin as executable (`chmod +x $IPFS_PATH/plugins/myplugin.so`).
3. Restart your IPFS daemon (if running).

### Preloaded Plugins

The advantages of preloaded plugins are:

1. They're bundled with the Emo binary.
2. They work on all platforms.

To preload a Emo plugin:

1. Add the plugin to the preload list: `plugin/loader/preload_list`
2. Build ipfs
```bash
emo$ make build
```

You can also preload an in-tree but disabled-by-default plugin by adding it to
the IPFS_PLUGINS variable. For example, to enable plugins foo, bar, and baz:

```bash
emo$ make build IPFS_PLUGINS="foo bar baz"
```

## Creating A Plugin

To create your own out-of-tree plugin, use the [example
plugin](https://github.com/ipfs/go-ipfs-example-plugin/) as a starting point.
When you're ready, submit a PR adding it to the list of [available
plugins](#available-plugins).
