package p2p

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

type Peer struct {
	IdForNetwork crypto.Hash
	Address      string

	sentMetric     *MetricPool
	receivedMetric *MetricPool

	ctx             context.Context
	handle          SyncHandle
	relayers        *neighborMap
	consumers       *neighborMap
	snapshotsCaches *confirmMap
	highRing        chan *ChanMsg
	normalRing      chan *ChanMsg
	syncRing        chan []*SyncPoint
	closing         bool
	ops             chan struct{}
	stn             chan struct{}

	relayer        *QuicRelayer
	consumerAuth   *AuthToken
	isRelayer      bool
	remoteRelayers *relayersMap
}

type SyncPoint struct {
	NodeId crypto.Hash `json:"node"`
	Number uint64      `json:"round"`
	Hash   crypto.Hash `json:"hash"`
	Pool   any         `json:"pool"`
}

type ChanMsg struct {
	key  []byte
	data []byte
}

func (me *Peer) IsRelayer() bool {
	return me.isRelayer
}

func (me *Peer) ConnectRelayer(idForNetwork crypto.Hash, addr string) {
	if a, err := net.ResolveUDPAddr("udp", addr); err != nil {
		panic(fmt.Errorf("invalid address %s %s", addr, err))
	} else if a.Port < 80 || a.IP == nil {
		panic(fmt.Errorf("invalid address %s %d %s", addr, a.Port, a.IP))
	}
	if me.isRelayer {
		me.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}
	}

	for !me.closing {
		time.Sleep(time.Duration(config.SnapshotRoundGap))
		old := me.relayers.Get(idForNetwork)
		if old != nil {
			panic(fmt.Errorf("ConnectRelayer(%s) => %s", idForNetwork, old.Address))
		}
		relayer := NewPeer(nil, idForNetwork, addr, true)
		err := me.connectRelayer(relayer)
		logger.Printf("me.connectRelayer(%s, %v) => %v", me.Address, relayer, err)
	}
}

func (me *Peer) connectRelayer(relayer *Peer) error {
	logger.Printf("me.connectRelayer(%s, %s) => %v", me.Address, me.IdForNetwork, relayer)
	client, err := NewQuicConsumer(me.ctx, relayer.Address)
	logger.Printf("NewQuicConsumer(%s) => %v %v", relayer.Address, client, err)
	if err != nil {
		return err
	}
	defer client.Close("connectRelayer")
	defer relayer.disconnect()

	auth := me.handle.BuildAuthenticationMessage(relayer.IdForNetwork)
	err = client.Send(buildAuthenticationMessage(auth))
	logger.Printf("client.SendAuthenticationMessage(%x) => %v", auth, err)
	if err != nil {
		return err
	}
	me.sentMetric.handle(PeerMessageTypeAuthentication)
	if !me.relayers.Put(relayer.IdForNetwork, relayer) {
		panic(fmt.Errorf("ConnectRelayer(%s) => %s", relayer.IdForNetwork, relayer.Address))
	}
	defer me.relayers.Delete(relayer.IdForNetwork)

	go me.syncToNeighborLoop(relayer)
	go me.loopReceiveMessage(relayer, client)
	_, err = me.loopSendingStream(relayer, client)
	logger.Printf("me.loopSendingStream(%s, %s) => %v", me.Address, client.RemoteAddr().String(), err)
	return err
}

func (me *Peer) Neighbors() []*Peer {
	relayers := me.relayers.Slice()
	consumers := me.consumers.Slice()
	return append(relayers, consumers...)
}

func (p *Peer) disconnect() {
	if p.closing {
		return
	}
	p.closing = true
	<-p.ops
	close(p.highRing)
	close(p.normalRing)
	close(p.syncRing)
	<-p.stn
}

func (me *Peer) Metric() map[string]*MetricPool {
	metrics := make(map[string]*MetricPool)
	if me.sentMetric.enabled {
		metrics["sent"] = me.sentMetric
	}
	if me.receivedMetric.enabled {
		metrics["received"] = me.receivedMetric
	}
	return metrics
}

func NewPeer(handle SyncHandle, idForNetwork crypto.Hash, addr string, isRelayer bool) *Peer {
	ringSize := uint64(1024)
	peer := &Peer{
		IdForNetwork:   idForNetwork,
		Address:        addr,
		relayers:       &neighborMap{m: make(map[crypto.Hash]*Peer)},
		consumers:      &neighborMap{m: make(map[crypto.Hash]*Peer)},
		highRing:       make(chan *ChanMsg, ringSize),
		normalRing:     make(chan *ChanMsg, ringSize),
		syncRing:       make(chan []*SyncPoint, 128),
		handle:         handle,
		sentMetric:     &MetricPool{enabled: false},
		receivedMetric: &MetricPool{enabled: false},
		ops:            make(chan struct{}),
		stn:            make(chan struct{}),
		isRelayer:      isRelayer,
	}
	peer.ctx = context.Background() // FIXME use real context
	if handle != nil {
		peer.snapshotsCaches = &confirmMap{cache: handle.GetCacheStore()}
	}
	return peer
}

func (me *Peer) Teardown() {
	me.closing = true
	if me.relayer != nil {
		me.relayer.Close()
	}
	close(me.highRing)
	close(me.normalRing)
	close(me.syncRing)
	peers := me.Neighbors()
	var wg sync.WaitGroup
	for _, p := range peers {
		wg.Add(1)
		go func(p *Peer) {
			p.disconnect()
			wg.Done()
		}(p)
	}
	wg.Wait()
	logger.Printf("Teardown(%s, %s)\n", me.IdForNetwork, me.Address)
}

func (me *Peer) ListenConsumers() error {
	logger.Printf("me.ListenConsumers(%s, %s)", me.Address, me.IdForNetwork)
	relayer, err := NewQuicRelayer(me.Address)
	if err != nil {
		return err
	}
	me.relayer = relayer
	me.remoteRelayers = &relayersMap{m: make(map[crypto.Hash][]*remoteRelayer)}

	go func() {
		ticker := time.NewTicker(time.Duration(config.SnapshotRoundGap))
		defer ticker.Stop()

		for !me.closing {
			neighbors := me.Neighbors()
			msg := me.buildConsumersMessage()
			for _, p := range neighbors {
				if !p.isRelayer {
					continue
				}
				me.offerToPeerWithCacheCheck(p, MsgPriorityNormal, &ChanMsg{nil, msg})
			}

			<-ticker.C
		}
	}()

	for !me.closing {
		c, err := me.relayer.Accept(me.ctx)
		logger.Printf("me.relayer.Accept(%s) => %v %v", me.Address, c, err)
		if err != nil {
			continue
		}
		go func(c Client) {
			defer c.Close("authenticateNeighbor")

			peer, err := me.authenticateNeighbor(c)
			logger.Printf("me.authenticateNeighbor(%s, %s) => %v %v", me.Address, c.RemoteAddr().String(), peer, err)
			if err != nil {
				return
			}
			defer peer.disconnect()

			old := me.consumers.Get(peer.IdForNetwork)
			if old != nil {
				old.disconnect()
				me.consumers.Delete(old.IdForNetwork)
			}
			if !me.consumers.Put(peer.IdForNetwork, peer) {
				panic(peer.IdForNetwork)
			}
			defer me.consumers.Delete(peer.IdForNetwork)

			go me.syncToNeighborLoop(peer)
			go me.loopReceiveMessage(peer, c)
			_, err = me.loopSendingStream(peer, c)
			logger.Printf("me.loopSendingStream(%s, %s) => %v", me.Address, c.RemoteAddr().String(), err)
		}(c)
	}

	logger.Printf("ListenConsumers(%s, %s) DONE\n", me.IdForNetwork, me.Address)
	return nil
}

func (me *Peer) loopSendingStream(p *Peer, consumer Client) (*ChanMsg, error) {
	defer close(p.ops)
	defer consumer.Close("loopSendingStream")

	for !me.closing && !p.closing {
		hm := me.pollRingWithCache(p.highRing, 16)
		nm := me.pollRingWithCache(p.normalRing, 16)
		msgs := append(hm, nm...)

		if len(msgs) == 0 {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		for _, m := range msgs {
			err := consumer.Send(m.data)
			if err != nil {
				return m, fmt.Errorf("consumer.Send(%s, %d) => %v", p.Address, len(m.data), err)
			}
			if m.key != nil {
				me.snapshotsCaches.store(m.key, time.Now())
			}
		}
	}

	return nil, fmt.Errorf("PEER DONE")
}

func (me *Peer) pollRingWithCache(ring chan *ChanMsg, limit int) []*ChanMsg {
	var msgs []*ChanMsg
	for len(msgs) < limit {
		select {
		case msg := <-ring:
			if me.snapshotsCaches.contains(msg.key, time.Minute) {
				continue
			}
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
	return msgs
}

func (me *Peer) loopReceiveMessage(peer *Peer, client Client) {
	logger.Printf("me.loopReceiveMessage(%s, %s)", me.Address, client.RemoteAddr().String())
	receive := make(chan *PeerMessage, 1024)
	defer close(receive)
	defer client.Close("loopReceiveMessage")

	go func() {
		defer client.Close("handlePeerMessage")

		for msg := range receive {
			err := me.handlePeerMessage(peer.IdForNetwork, msg)
			if err == nil {
				continue
			}
			logger.Printf("me.handlePeerMessage(%s) => %v", peer.IdForNetwork, err)
			return
		}
	}()

	for !me.closing {
		tm, err := client.Receive()
		if err != nil {
			logger.Printf("client.Receive %s %v", peer.Address, err)
			return
		}
		msg, err := parseNetworkMessage(tm.Version, tm.Data)
		if err != nil {
			logger.Debugf("parseNetworkMessage %s %v", peer.Address, err)
			return
		}
		me.receivedMetric.handle(msg.Type)

		select {
		case receive <- msg:
		default:
			logger.Printf("peer receive timeout %s", peer.Address)
			return
		}
	}
}

func (me *Peer) authenticateNeighbor(client Client) (*Peer, error) {
	var peer *Peer
	auth := make(chan error)
	go func() {
		tm, err := client.Receive()
		if err != nil {
			auth <- err
			return
		}
		msg, err := parseNetworkMessage(tm.Version, tm.Data)
		if err != nil {
			auth <- err
			return
		}
		if msg.Type != PeerMessageTypeAuthentication {
			auth <- fmt.Errorf("peer authentication invalid message type %d", msg.Type)
			return
		}
		me.receivedMetric.handle(PeerMessageTypeAuthentication)

		token, err := me.handle.AuthenticateAs(me.IdForNetwork, msg.Data, int64(HandshakeTimeout/time.Second))
		if err != nil {
			auth <- err
			return
		}

		addr := client.RemoteAddr().String()
		peer = NewPeer(nil, token.PeerId, addr, token.IsRelayer)
		peer.consumerAuth = token
		auth <- nil
	}()

	select {
	case err := <-auth:
		if err != nil {
			return nil, err
		}
	case <-time.After(3 * time.Second):
		return nil, fmt.Errorf("authenticate timeout")
	}
	return peer, nil
}

func (me *Peer) sendHighToPeer(to crypto.Hash, typ byte, key, data []byte) error {
	return me.sendToPeer(to, typ, key, data, MsgPriorityHigh)
}

func (me *Peer) offerToPeerWithCacheCheck(p *Peer, priority int, msg *ChanMsg) bool {
	if p.IdForNetwork == me.IdForNetwork {
		return true
	}
	if me.snapshotsCaches.contains(msg.key, time.Minute) {
		return true
	}
	return p.offer(priority, msg)
}

func (p *Peer) offer(priority int, msg *ChanMsg) bool {
	if p.closing {
		return false
	}
	switch priority {
	case MsgPriorityNormal:
		select {
		case p.normalRing <- msg:
			return true
		default:
			return false
		}
	case MsgPriorityHigh:
		select {
		case p.highRing <- msg:
			return true
		default:
			return false
		}
	}
	panic(priority)
}

func (me *Peer) sendToPeer(to crypto.Hash, typ byte, key, data []byte, priority int) error {
	if to == me.IdForNetwork {
		return nil
	}
	if me.snapshotsCaches.contains(key, time.Minute) {
		return nil
	}
	me.sentMetric.handle(typ)

	nbrs := me.GetNeighbors(to)
	if len(nbrs) > 0 {
		for _, peer := range nbrs {
			success := peer.offer(priority, &ChanMsg{key, data})
			if !success {
				logger.Verbosef("peer.offer(%s) send timeout\n", peer.IdForNetwork)
			}
		}
		return nil
	}

	rm := me.buildRelayMessage(to, data)
	rk := crypto.Blake3Hash(rm)
	rk = crypto.Blake3Hash(append(rk[:], []byte("REMOTE")...))
	relayers := me.GetRemoteRelayers(to)
	if len(relayers) == 0 {
		relayers = me.relayers.Slice()
	}
	for _, peer := range relayers {
		if !peer.IsRelayer() {
			panic(peer.IdForNetwork)
		}
		rk := crypto.Blake3Hash(append(rk[:], peer.IdForNetwork[:]...))
		success := me.offerToPeerWithCacheCheck(peer, priority, &ChanMsg{rk[:], rm})
		if !success {
			logger.Verbosef("me.offerToPeerWithCacheCheck(%s) send timeout\n", peer.IdForNetwork)
		}
	}
	return nil
}

func (me *Peer) sendSnapshotMessageToPeer(to crypto.Hash, snap crypto.Hash, typ byte, data []byte) error {
	key := append(to[:], snap[:]...)
	key = append(key, 'S', 'N', 'A', 'P', typ)
	return me.sendToPeer(to, typ, key, data, MsgPriorityNormal)
}

func (me *Peer) GetNeighbors(key crypto.Hash) []*Peer {
	var nbrs []*Peer
	p := me.relayers.Get(key)
	if p != nil {
		nbrs = append(nbrs, p)
	}
	p = me.consumers.Get(key)
	if p != nil {
		nbrs = append(nbrs, p)
	}
	return nbrs
}

func (me *Peer) GetRemoteRelayers(key crypto.Hash) []*Peer {
	if me.remoteRelayers == nil {
		return nil
	}
	var relayers []*Peer
	ids := me.remoteRelayers.Get(key)
	for _, id := range ids {
		nbrs := me.GetNeighbors(id)
		relayers = append(relayers, nbrs...)
	}
	return relayers
}
