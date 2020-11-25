package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
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

	nodes := source.ReadAllNodes(uint64(clock.Now().UnixNano()), false)
	for _, cn := range nodes {
		id := cn.IdForNetwork(node.networkId)
		chain := node.GetOrCreateChain(id)
		go func(chain *Chain) {
			total, err := chain.importFrom(source)
			logger.Printf("NODE %s IMPORT FINISHED WITH %d %v\n", id, total, err)
		}(chain)
	}

	startAt := clock.Now()
	for {
		time.Sleep(10 * time.Second)
		duration := clock.Now().Sub(startAt).Seconds()
		sps := float64(node.TopoCounter.seq) / duration
		logger.Printf("TOPO %d SPS ALL %f LIVE %f\n", node.TopoCounter.seq, sps, node.TopoCounter.sps)
	}
}

func (chain *Chain) importFrom(source storage.Store) (uint64, error) {
	for i := uint64(0); ; i++ {
		ss, err := source.ReadSnapshotsForNodeRound(chain.ChainId, i)
		if err != nil || len(ss) == 0 {
			return i, err
		}
		for _, s := range ss {
			tx, _, err := source.ReadTransaction(s.Transaction)
			if err != nil {
				return i, err
			}
			err = chain.importSnapshot(s, tx)
			if err != nil {
				return i, err
			}
		}
	}
}

func (chain *Chain) importSnapshot(s *common.SnapshotWithTopologicalOrder, tx *common.VersionedTransaction) error {
	if s.Transaction != tx.PayloadHash() {
		return fmt.Errorf("malformed transaction hash %s %s", s.Transaction, tx.PayloadHash())
	}
	old, _, err := chain.persistStore.ReadTransaction(s.Transaction)
	if err != nil {
		return fmt.Errorf("ReadTransaction %s %v", s.Transaction, err)
	} else if old == nil {
		err := chain.persistStore.CachePutTransaction(tx)
		if err != nil {
			return fmt.Errorf("CachePutTransaction %s %v", s.Transaction, err)
		}
	}

	for {
		err = chain.AppendFinalSnapshot(chain.node.IdForNetwork, &s.Snapshot)
		if err != nil {
			time.Sleep(3 * time.Second)
		} else {
			return nil
		}
	}
}
