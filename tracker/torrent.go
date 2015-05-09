package tracker

import (
	"fmt"
	"git.totdev.in/totv/mika/db"
	"git.totdev.in/totv/mika/util"
	log "github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"strings"
	"sync"
)

type Torrent struct {
	db.Queued
	sync.RWMutex
	Name            string  `redis:"name" json:"name"`
	TorrentID       uint64  `redis:"torrent_id" json:"torrent_id"`
	InfoHash        string  `redis:"info_hash" json:"info_hash"`
	Seeders         int16   `redis:"seeders" json:"seeders"`
	Leechers        int16   `redis:"leechers" json:"leechers"`
	Snatches        int16   `redis:"snatches" json:"snatches"`
	Announces       uint64  `redis:"announces" json:"announces"`
	Uploaded        uint64  `redis:"uploaded" json:"uploaded"`
	Downloaded      uint64  `redis:"downloaded" json:"downloaded"`
	TorrentKey      string  `redis:"-" json:"-"`
	TorrentPeersKey string  `redis:"-" json:"-"`
	Enabled         bool    `redis:"enabled" json:"enabled"`
	Reason          string  `redis:"reason" json:"reason"`
	Peers           []*Peer `redis:"-" json:"-"`
	MultiUp         float64 `redis:"multi_up" json:"-"`
	MultiDn         float64 `redis:"multi_dn" json:"-"`
}

func NewTorrent(info_hash string, name string, torrent_id uint64) *Torrent {
	torrent := &Torrent{
		Name:            name,
		InfoHash:        strings.ToLower(info_hash),
		TorrentKey:      fmt.Sprintf("t:t:%s", info_hash),
		TorrentPeersKey: fmt.Sprintf("t:tpeers:%s", info_hash),
		Enabled:         true,
		Peers:           []*Peer{},
		TorrentID:       torrent_id,
		MultiUp:         1.0,
		MultiDn:         1.0,
	}
	return torrent
}

func (torrent *Torrent) MergeDB(r redis.Conn) error {
	torrent_reply, err := r.Do("HGETALL", torrent.TorrentKey)
	if err != nil {
		log.Println(fmt.Sprintf("FetchTorrent: Failed to get torrent from redis [%s]", torrent.TorrentKey), err)
		return err
	}

	values, err := redis.Values(torrent_reply, nil)
	if err != nil {
		log.Println("FetchTorrent: Failed to parse torrent reply: ", err)
		return err
	}

	err = redis.ScanStruct(values, torrent)
	if err != nil {
		log.Println("FetchTorrent: Torrent scanstruct failure", err)
		return err
	}

	if torrent.TorrentID == 0 {
		log.Debug("FetchTorrent: Trying to fetch info hash without valid key:", torrent.InfoHash)
		r.Do("DEL", fmt.Sprintf("t:t:%s", torrent.InfoHash))
	}
	return nil
}

func (torrent *Torrent) Update(announce *AnnounceRequest, upload_diff, download_diff uint64) {
	s, l := torrent.PeerCounts()
	torrent.Lock()
	torrent.Announces++
	torrent.Uploaded += upload_diff
	torrent.Downloaded += download_diff
	torrent.Seeders = s
	torrent.Leechers = l
	if announce.Event == COMPLETED {
		torrent.Snatches++
	}
	torrent.Unlock()
}

func (torrent *Torrent) Sync(r redis.Conn) {
	r.Send(
		"HMSET", torrent.TorrentKey,
		"torrent_id", torrent.TorrentID,
		"seeders", torrent.Seeders,
		"leechers", torrent.Leechers,
		"snatches", torrent.Snatches,
		"announces", torrent.Announces,
		"uploaded", torrent.Uploaded,
		"downloaded", torrent.Downloaded,
		"info_hash", torrent.InfoHash,
		"reason", torrent.Reason,
		"enabled", torrent.Enabled,
		"name", torrent.Name,
		"multi_up", torrent.MultiUp,
		"multi_dn", torrent.MultiDn,
	)
}

func (torrent *Torrent) findPeer(peer_id string) *Peer {
	torrent.RLock()
	defer torrent.RUnlock()
	for _, peer := range torrent.Peers {
		if peer.PeerID == peer_id {
			return peer
		}
	}
	return nil
}

func (torrent *Torrent) Delete(reason string) {
	torrent.Enabled = false
	torrent.Reason = reason
}

func (torrent *Torrent) DelReason() string {
	if torrent.Reason == "" {
		return "Torrent deleted"
	} else {
		return torrent.Reason
	}
}

// Add a peer to a torrents active peer_id list
func (torrent *Torrent) AddPeer(r redis.Conn, peer *Peer) bool {
	torrent.Lock()
	torrent.Peers = append(torrent.Peers, peer)
	torrent.Unlock()
	v, err := r.Do("SADD", torrent.TorrentPeersKey, peer.PeerID)
	if err != nil {
		log.Println("AddPeer: Error executing peer fetch query: ", err)
		return false
	}
	if v == "0" {
		log.Println("AddPeer: Tried to add peer to set with existing element")
	}
	return true
}

// Remove a peer from a torrents active peer_id list
func (torrent *Torrent) DelPeer(r redis.Conn, peer *Peer) bool {
	torrent.RLock()
	defer torrent.RUnlock()
	for i, tor_peer := range torrent.Peers {
		if tor_peer == peer {
			if len(torrent.Peers) == 1 {
				torrent.Peers = nil
			} else {
				torrent.Peers = append(torrent.Peers[:i], torrent.Peers[i+1:]...)
			}
			break
		}
	}

	r.Send("SREM", torrent.TorrentPeersKey, peer.PeerID)

	peer.Lock()
	peer.Active = false
	peer.Unlock()
	return true
}

// HasPeer Checks if the peer already is a member of the peer slice for the torrent
func (torrent *Torrent) HasPeer(peer *Peer) bool {
	for _, p := range torrent.Peers {
		if peer == p {
			return true
		}
	}
	return false
}

// PeerCounts counts the number of seeders and leechers the torrent currently has.
// Only active peers are counted towards the totals
func (torrent *Torrent) PeerCounts() (int16, int16) {
	s, l := 0, 0
	torrent.RLock()
	defer torrent.RUnlock()
	for _, p := range torrent.Peers {
		if p.Active {
			if p.IsSeeder() {
				s++
			} else {
				l++
			}
		}
	}
	return int16(s), int16(l)
}

// Get an array of peers for the torrent
func (torrent *Torrent) GetPeers(r redis.Conn, max_peers int) []*Peer {
	return torrent.Peers[0:util.UMin(uint64(len(torrent.Peers)), uint64(max_peers))]
}
