package node

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"go.uber.org/fx"

	"github.com/ipfs/emo/core/node/helpers"
	"github.com/ipfs/emo/core/node/libp2p"
	"github.com/ipfs/emo/repo"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	cfg "github.com/ipfs/emo/config"
	"github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

type BuildCfg struct {
	// If online is set, the node will have networking enabled
	Online bool

	// ExtraOpts is a map of extra options used to configure the ipfs nodes creation
	ExtraOpts map[string]bool

	// If permanent then node should run more expensive processes
	// that will improve performance in long run
	Permanent bool

	// DisableEncryptedConnections disables connection encryption *entirely*.
	// DO NOT SET THIS UNLESS YOU'RE TESTING.
	DisableEncryptedConnections bool

	Routing libp2p.RoutingOption
	Host    libp2p.HostOption
	Repo    repo.Repo
}

func (cfg *BuildCfg) getOpt(key string) bool {
	if cfg.ExtraOpts == nil {
		return false
	}

	return cfg.ExtraOpts[key]
}

func (cfg *BuildCfg) fillDefaults() error {
	if cfg.Repo == nil {
		r, err := defaultRepo(dsync.MutexWrap(ds.NewMapDatastore()))
		if err != nil {
			return err
		}
		cfg.Repo = r
	}

	if cfg.Routing == nil {
		cfg.Routing = libp2p.DHTOption
	}

	if cfg.Host == nil {
		cfg.Host = libp2p.DefaultHostOption
	}

	return nil
}

// options creates fx option group from this build config
func (cfg *BuildCfg) options(ctx context.Context) (fx.Option, *cfg.Config) {
	err := cfg.fillDefaults()
	if err != nil {
		return fx.Error(err), nil
	}

	repoOption := fx.Provide(func(lc fx.Lifecycle) repo.Repo {
		lc.Append(fx.Hook{
			OnStop: func(ctx context.Context) error {
				return cfg.Repo.Close()
			},
		})

		return cfg.Repo
	})

	metricsCtx := fx.Provide(func() helpers.MetricsCtx {
		return helpers.MetricsCtx(ctx)
	})

	hostOption := fx.Provide(func() libp2p.HostOption {
		return cfg.Host
	})

	routingOption := fx.Provide(func() libp2p.RoutingOption {
		return cfg.Routing
	})

	conf, err := cfg.Repo.Config()
	if err != nil {
		return fx.Error(err), nil
	}

	return fx.Options(
		repoOption,
		hostOption,
		routingOption,
		metricsCtx,
	), conf
}

func defaultRepo(dstore repo.Datastore) (repo.Repo, error) {
	c := cfg.Config{}
	priv, pub, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
	if err != nil {
		return nil, err
	}

	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, err
	}

	privkeyb, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, err
	}

	c.Bootstrap = cfg.DefaultBootstrapAddresses
	c.Addresses.Swarm = []string{"/ip4/0.0.0.0/tcp/4001", "/ip4/0.0.0.0/udp/4001/quic-v1"}
	c.Identity.PeerID = pid.String()
	c.Identity.PrivKey = base64.StdEncoding.EncodeToString(privkeyb)

	return &repo.Mock{
		D: dstore,
		C: c,
	}, nil
}
