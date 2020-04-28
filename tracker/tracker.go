package tracker

import (
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"mika/config"
	"mika/geo"
	"mika/store"
	"sync"
)

type Tracker struct {
	torrents store.TorrentStore
	peers    store.PeerStore
	users    store.UserStore
	geodb    *geo.DB
	// Whitelist and whitelist lock
	WhitelistMutex *sync.RWMutex
	Whitelist      []string
}

func New() (*Tracker, error) {
	var err error
	s, err := store.NewTorrentStore(
		viper.GetString(config.StoreTorrentType),
		config.GetStoreConfig(config.Torrent))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to setup torrent store")
	}
	p, err := store.NewPeerStore(viper.GetString(config.StorePeersType),
		config.GetStoreConfig(config.Peers))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to setup peer store")
	}
	u, err := store.NewUserStore(viper.GetString(config.StorePeersType),
		config.GetStoreConfig(config.Peers))
	if err != nil {
		return nil, errors.Wrap(err, "Failed to setup user store")
	}
	geodb := geo.New(viper.GetString(config.GeodbPath))
	return &Tracker{
		torrents: s,
		peers:    p,
		users:    u,
		geodb:    geodb,
	}, nil
}
