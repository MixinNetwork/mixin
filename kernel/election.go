package kernel

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
)

func (node *Node) reloadConsensusNodesList(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if tx.TransactionType() == common.TransactionTypeNodeAccept {
		gns, err := readGenesis(node.configDir + "/genesis.json")
		if err != nil {
			return err
		}
		matchId := gns.Nodes[0].Signer.Hash().ForNetwork(node.networkId)
		distance := nodeDistance(s.NodeId, matchId)
		for _, n := range gns.Nodes {
			id := n.Signer.Hash().ForNetwork(node.networkId)
			if nd := nodeDistance(s.NodeId, id); nd < distance {
				distance = nd
				matchId = id
			}
		}

		var link common.RoundLink
		ss, err := node.store.ReadSnapshotsForNodeRound(matchId, 0)
		if err != nil {
			return err
		}
		rss := make([]*common.Snapshot, len(ss))
		for i, s := range ss {
			rss[i] = &s.Snapshot
		}
		_, _, link.External = ComputeRoundHash(matchId, 0, rss)
		_, _, link.Self = ComputeRoundHash(s.NodeId, 0, []*common.Snapshot{s})
		err = node.store.StartNewRound(s.NodeId, 1, &link, s.Timestamp+config.SnapshotRoundGap+1)
		if err != nil {
			return err
		}
	}

	switch tx.TransactionType() {
	case common.TransactionTypeNodePledge, common.TransactionTypeNodeAccept, common.TransactionTypeNodeDepart, common.TransactionTypeNodeRemove:
		err := node.LoadConsensusNodes()
		if err != nil {
			return err
		}
		graph, err := LoadRoundGraph(node.store, node.networkId, node.IdForNetwork)
		if err != nil {
			return err
		}
		node.Graph = graph
	}
	return nil
}

func nodeDistance(a, b crypto.Hash) int {
	ai := new(big.Int).SetBytes(a[:])
	bi := new(big.Int).SetBytes(b[:])
	si := new(big.Int).Sub(ai, bi)
	ai = new(big.Int).Abs(si)
	mi := new(big.Int).Mod(ai, big.NewInt(100))
	return int(mi.Int64())
}

func (node *Node) validateNodePledgeSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	for _, cn := range node.ConsensusNodes {
		if s.Timestamp < cn.Timestamp {
			return fmt.Errorf("invalid snapshot timestamp %d %d", cn.Timestamp, s.Timestamp)
		}
		elapse := time.Duration(s.Timestamp - cn.Timestamp)
		if elapse < config.KernelNodePledgePeriodMinimum {
			return fmt.Errorf("invalid pledge period %d %d", config.KernelNodePledgePeriodMinimum, elapse)
		}
		if cn.State != common.NodeStateAccepted {
			return fmt.Errorf("invalid node state %s %s", cn.Signer, cn.State)
		}
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if s.Timestamp > node.Graph.GraphTimestamp+threshold*2 {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Graph.GraphTimestamp, s.Timestamp)
	}
	if cn := node.ConsensusPledging; cn != nil {
		return fmt.Errorf("invalid node state %s %s", cn.Signer, cn.State)
	}
	if tx.Asset != common.XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for pledge transaction", len(tx.Outputs))
	}
	if len(tx.Extra) != 2*len(crypto.Key{}) {
		return fmt.Errorf("invalid extra length %d for pledge transaction", len(tx.Extra))
	}
	if tx.Outputs[0].Amount.Cmp(common.NewInteger(10000)) != 0 {
		return fmt.Errorf("invalid pledge amount %s", tx.Outputs[0].Amount.String())
	}

	return node.store.AddNodeOperation(tx, s.Timestamp, uint64(config.KernelNodeOperationLockThreshold))
}

func (node *Node) validateNodeAcceptSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if tx.Asset != common.XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for accept transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for accept transaction", len(tx.Inputs))
	}
	if node.ConsensusPledging == nil {
		return fmt.Errorf("invalid consensus status")
	}
	if id := node.ConsensusPledging.Signer.Hash().ForNetwork(node.networkId); id != s.NodeId {
		return fmt.Errorf("invalid pledging node %s %s", id, s.NodeId)
	}
	if node.ConsensusPledging.Transaction != tx.Inputs[0].Hash {
		return fmt.Errorf("invalid plede utxo source %s %s", node.ConsensusPledging.Transaction, tx.Inputs[0].Hash)
	}

	pledge, err := node.store.ReadTransaction(tx.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if len(pledge.Outputs) != 1 {
		return fmt.Errorf("invalid pledge utxo count %d", len(pledge.Outputs))
	}
	if pledge.Outputs[0].Type != common.OutputTypeNodePledge {
		return fmt.Errorf("invalid pledge utxo type %d", pledge.Outputs[0].Type)
	}
	if bytes.Compare(pledge.Extra, tx.Extra) != 0 {
		return fmt.Errorf("invalid pledge and accpet key %s %s", hex.EncodeToString(pledge.Extra), hex.EncodeToString(tx.Extra))
	}

	if s.RoundNumber != 0 {
		return fmt.Errorf("invalid snapshot round %d", s.RoundNumber)
	}
	if s.Timestamp < node.epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.epoch, s.Timestamp)
	}
	if r := node.Graph.CacheRound[s.NodeId]; r != nil {
		return fmt.Errorf("invalid graph round %s %d", s.NodeId, r.Number)
	}
	if r := node.Graph.FinalRound[s.NodeId]; r != nil {
		return fmt.Errorf("invalid graph round %s %d", s.NodeId, r.Number)
	}

	since := s.Timestamp - node.epoch
	hours := int(since / 3600000000000)
	if hours%24 < config.KernelNodeAcceptTimeBegin || hours%24 > config.KernelNodeAcceptTimeEnd {
		return fmt.Errorf("invalid node accept hour %d", hours%24)
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if s.Timestamp+threshold*2 < node.Graph.GraphTimestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Graph.GraphTimestamp, s.Timestamp)
	}

	if s.Timestamp < node.ConsensusPledging.Timestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.ConsensusPledging.Timestamp, s.Timestamp)
	}
	elapse := time.Duration(s.Timestamp - node.ConsensusPledging.Timestamp)
	if elapse > config.KernelNodeAcceptPeriodMaximum {
		return fmt.Errorf("invalid accept period %d %d", config.KernelNodeAcceptPeriodMaximum, elapse)
	}

	return nil
}
