package coremock

import (
	"context"
	"fmt"
	"io"

	libp2p2 "github.com/ipfs/emo/core/node/libp2p"

	"github.com/ipfs/emo/commands"
	"github.com/ipfs/emo/core"
	"github.com/ipfs/emo/repo"

	"github.com/ipfs/go-datastore"
	syncds "github.com/ipfs/go-datastore/sync"
	config "github.com/ipfs/emo/config"

	"github.com/libp2p/go-libp2p"
	testutil "github.com/libp2p/go-libp2p-testing/net"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	pstore "github.com/libp2p/go-libp2p/core/peerstore"

	mocknet "github.com/libp2p/go-libp2p/p2p/net/mock"
)

// NewMockNode constructs an IpfsNode for use in tests.
func NewMockNode() (*core.IpfsNode, error) {
	// effectively offline, only peer in its network
	return core.NewNode(context.Background(), &core.BuildCfg{
		Online: true,
		Host:   MockHostOption(mocknet.New()),
	})
}

func MockHostOption(mn mocknet.Mocknet) libp2p2.HostOption {
	return func(id peer.ID, ps pstore.Peerstore, opts ...libp2p.Option) (host.Host, error) {
		var cfg libp2p.Config
		if err := cfg.Apply(opts...); err != nil {
			return nil, err
		}

		// The mocknet does not use the provided libp2p.Option. This options include
		// the listening addresses we want our peer listening on. Therefore, we have
		// to manually parse the configuration and add them here.
		ps.AddAddrs(id, cfg.ListenAddrs, pstore.PermanentAddrTTL)
		return mn.AddPeerWithPeerstore(id, ps)
	}
}

func MockCmdsCtx() (commands.Context, error) {
	// Generate Identity
	ident, err := testutil.RandIdentity()
	if err != nil {
		return commands.Context{}, err
	}
	p := ident.ID()

	conf := config.Config{
		Identity: config.Identity{
			PeerID: p.String(),
		},
	}

	r := &repo.Mock{
		D: syncds.MutexWrap(datastore.NewMapDatastore()),
		C: conf,
	}

	node, err := core.NewNode(context.Background(), &core.BuildCfg{
		Repo: r,
	})
	if err != nil {
		return commands.Context{}, err
	}

	return commands.Context{
		ConfigRoot: "/tmp/.mockipfsconfig",
		ConstructNode: func() (*core.IpfsNode, error) {
			return node, nil
		},
	}, nil
}

func MockPublicNode(ctx context.Context, mn mocknet.Mocknet) (*core.IpfsNode, error) {
	ds := syncds.MutexWrap(datastore.NewMapDatastore())
	cfg, err := config.Init(io.Discard, 2048)
	if err != nil {
		return nil, err
	}
	count := len(mn.Peers())
	cfg.Addresses.Swarm = []string{
		fmt.Sprintf("/ip4/18.0.%d.%d/tcp/4001", count>>16, count&0xFF),
	}
	cfg.Datastore = config.Datastore{}
	return core.NewNode(ctx, &core.BuildCfg{
		Online:  true,
		Routing: libp2p2.DHTServerOption,
		Repo: &repo.Mock{
			C: *cfg,
			D: ds,
		},
		Host: MockHostOption(mn),
	})
}
