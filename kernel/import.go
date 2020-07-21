package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/MixinNetwork/mixin/network"
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

	done := make(chan struct{})
	defer close(done)

	go node.importAllNodeHeads(source, done)

	var latestSnapshots []*common.SnapshotWithTopologicalOrder
	offset, limit := uint64(0), uint64(500)
	startAt := time.Now().Unix()
	for {
		snapshots, transactions, err := source.ReadSnapshotWithTransactionsSinceTopology(offset, limit)
		if err != nil {
			logger.Printf("source.ReadSnapshotWithTransactionsSinceTopology(%d, %d) %v\n", offset, limit, err)
		}

		for i, s := range snapshots {
			err := node.importSnapshot(s, transactions[i])
			if err != nil {
				return err
			}
		}

		if len(snapshots) > 0 {
			offset += limit
			latestSnapshots = snapshots
			s := snapshots[0]
			ts := time.Unix(0, int64(s.Timestamp)).Format(time.RFC3339)
			sps := float64(offset) / float64(time.Now().Unix()-startAt)
			logger.Printf("PROGRESS %d\t%s\t%f\n", s.TopologicalOrder, ts, sps)
		}

		if uint64(len(snapshots)) != limit {
			logger.Printf("source.ReadSnapshotWithTransactionsSinceTopology(%d, %d) DONE %d\n", offset, limit, len(snapshots))
			break
		}
	}

	for {
		time.Sleep(1 * time.Minute)
		var pending bool
		for _, s := range latestSnapshots {
			ss, err := node.persistStore.ReadSnapshot(s.Hash)
			if err != nil || ss == nil {
				logger.Printf("store.ReadSnapshot(%s) %v %v\n", s.Hash, ss, err)
				pending = true
				break
			}
		}
		if !pending {
			break
		}
	}

	return nil
}

func (node *Node) importSnapshot(s *common.SnapshotWithTopologicalOrder, tx *common.VersionedTransaction) error {
	if s.Transaction != tx.PayloadHash() {
		return fmt.Errorf("malformed transaction hash %s %s", s.Transaction, tx.PayloadHash())
	}
	old, finalized, err := node.persistStore.ReadTransaction(s.Transaction)
	if err != nil {
		return fmt.Errorf("ReadTransaction %s %v", s.Transaction, err)
	} else if finalized != "" {
		return nil
	} else if old == nil {
		err := node.persistStore.CachePutTransaction(tx)
		if err != nil {
			return fmt.Errorf("CachePutTransaction %s %v", s.Transaction, err)
		}
	}

	chain := node.GetOrCreateChain(s.NodeId)
	for {
		err = chain.AppendFinalSnapshot(node.IdForNetwork, &s.Snapshot)
		if err != nil {
			logger.Printf("QueueAppendSnapshot %s %v\n", s.Transaction, err)
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}
	return nil
}

func (node *Node) importAllNodeHeads(source storage.Store, done chan struct{}) error {
	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()

	nodes := source.ReadAllNodes()

	for {
		select {
		case <-done:
			return nil
		case <-ticker.C:
			graph := node.BuildGraph()
			for _, n := range nodes {
				id := n.IdForNetwork(node.networkId)
				node.importNodeHead(source, graph, id)
			}
		}
	}
}

func (node *Node) importNodeHead(source storage.Store, graph []*network.SyncPoint, id crypto.Hash) {
	var remoteFinal uint64
	for _, sp := range graph {
		if sp.NodeId == id {
			remoteFinal = sp.Number
		}
	}
	for i := remoteFinal; i <= remoteFinal+config.SnapshotSyncRoundThreshold*2; i++ {
		ss, _ := source.ReadSnapshotsForNodeRound(id, i)
		for _, s := range ss {
			tx, _, _ := source.ReadTransaction(s.Transaction)
			node.importSnapshot(s, tx)
		}
	}
}
