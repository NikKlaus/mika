package mysql

import (
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"mika/consts"
	"mika/model"
	"mika/store"
)

type PeerStore struct {
	db *sqlx.DB
}

func (ps *PeerStore) Close() error {
	return ps.db.Close()
}

func (ps *PeerStore) UpdatePeer(t *model.Torrent, p *model.Peer) error {
	panic("implement me")
}

func (ps *PeerStore) AddPeer(t *model.Torrent, p *model.Peer) error {
	const q = `
	INSERT INTO peers 
	    (peer_id, torrent_id, addr_ip, addr_port, location, user_id, created_on, updated_on)
	VALUES 
	    (:peer_id, :torrent_id, :addr_ip, :addr_port, :location, :user_id, now(), :updated_on)
	`
	res, err := ps.db.Exec(q, p.PeerId, t.TorrentID, p.IP, p.Port, p.Location, p.UserId)
	if err != nil {
		return err
	}
	lastId, err := res.LastInsertId()
	if err != nil {
		return errors.New("Failed to fetch insert ID")
	}
	p.UserPeerId = uint32(lastId)
	return nil
}

func (ps *PeerStore) DeletePeer(t *model.Torrent, p *model.Peer) error {
	const q = `DELETE FROM peers WHERE user_peer_id = :user_peer_id`
	_, err := ps.db.NamedExec(q, p)
	return err
}

func (ps *PeerStore) GetPeers(t *model.Torrent, limit int) ([]*model.Peer, error) {
	const q = `SELECT * FROM peers WHERE torrent_id = ? LIMIT ?`
	var peers []*model.Peer
	if err := ps.db.Select(&peers, q, t.TorrentID, limit); err != nil {
		return nil, err
	}
	return peers, nil
}

func (ps *PeerStore) GetScrape(t *model.Torrent) {
	panic("implement me")
}

type peerDriver struct{}

func (pd peerDriver) NewPeerStore(cfg interface{}) (store.PeerStore, error) {
	c, ok := cfg.(*store.SQLConfig)
	if !ok {
		return nil, consts.ErrInvalidConfig
	}
	var db *sqlx.DB
	if c.Conn != nil {
		db = c.Conn
	} else {
		db = sqlx.MustConnect("mysql", c.DSN())
	}
	return &PeerStore{
		db: db,
	}, nil
}

func init() {
	store.AddPeerDriver(driverName, peerDriver{})
}
