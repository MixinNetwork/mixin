package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/storage"
)

func (node *Node) Import(configDir string, source storage.Store) error {
	gns, err := readGenesis(configDir + "/genesis.json")
	if err != nil {
		return err
	}
	_, gss, _, err := buildGenesisSnapshots(node.networkId, node.Epoch, gns)
	if err != nil {
		return err
	}
	kss, err := node.persistStore.ReadSnapshotsSinceTopology(0, 100)
	if err != nil {
		return err
	}
	if len(gss) != len(kss) {
		return fmt.Errorf("kernel already initilaized %d %d", len(gss), len(kss))
	}

	for i, gs := range gss {
		ks := kss[i]
		if ks.PayloadHash() != gs.PayloadHash() {
			return fmt.Errorf("kernel genesis unmatch %d %s %s", i, gs.PayloadHash(), ks.PayloadHash())
		}
	}

	go func() {
		startAt := clock.Now()
		for {
			time.Sleep(10 * time.Second)
			duration := clock.Now().Sub(startAt).Seconds()
			sps := float64(node.TopoCounter.seq) / duration
			logger.Printf("TOPO %d SPS ALL %f LIVE %f\n", node.TopoCounter.seq, sps, node.TopoCounter.sps)
		}
	}()

	waiting := make(chan error)
	nodes := source.ReadAllNodes(uint64(clock.Now().UnixNano()), false)
	for _, cn := range nodes {
		id := cn.IdForNetwork(node.networkId)
		cn := node.GetOrCreateChain(id)
		go func(chain *Chain) {
			total, err := chain.importFrom(source)
			logger.Printf("NODE %s IMPORT FINISHED WITH %d %v\n", id, total, err)
			waiting <- err
		}(cn)
	}

	for i := 0; i < len(nodes); i++ {
		<-waiting
	}
	return nil
}

func (chain *Chain) importFrom(source storage.Store) (uint64, error) {
	var threshold, round uint64
	filter := make(map[uint64]time.Time)
	period := time.Duration(config.SnapshotRoundGap)
	for {
		if cs := chain.State; cs != nil {
			threshold = cs.CacheRound.Number
		}
		if round > threshold+128 {
			time.Sleep(period)
			round = threshold
		}
		if filter[round].Add(period * 2).After(clock.Now()) {
			continue
		}
		filter[round] = clock.Now()
		ss, err := source.ReadSnapshotsForNodeRound(chain.ChainId, round)
		if err != nil || len(ss) == 0 {
			return round, err
		}
		for _, s := range ss {
			tx, _, err := source.ReadTransaction(s.Transaction)
			if err != nil {
				return round, err
			}
			err = chain.importSnapshot(s, tx)
			if err != nil {
				return round, err
			}
		}
		round = round + 1
	}
}

func (chain *Chain) importSnapshot(s *common.SnapshotWithTopologicalOrder, tx *common.VersionedTransaction) error {
	if s.Transaction != tx.PayloadHash() {
		return fmt.Errorf("malformed transaction hash %s %s", s.Transaction, tx.PayloadHash())
	}
	old, err := chain.persistStore.CacheGetTransaction(s.Transaction)
	if err != nil {
		return fmt.Errorf("ReadTransaction %s %v", s.Transaction, err)
	}
	if old == nil {
		err := chain.persistStore.CachePutTransaction(tx)
		if err != nil {
			return fmt.Errorf("CachePutTransaction %s %v", s.Transaction, err)
		}
	}

	for {
		err = chain.AppendFinalSnapshot(chain.node.IdForNetwork, s.Snapshot)
		if err != nil {
			time.Sleep(3 * time.Second)
		} else {
			return nil
		}
	}
}
