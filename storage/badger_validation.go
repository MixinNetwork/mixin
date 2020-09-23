package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (s *BadgerStore) ValidateGraphEntries(networkId crypto.Hash, depth uint64) (int, int, error) {
	nodes := s.ReadAllNodes(uint64(time.Now().UnixNano()), false)
	stats := make(chan [2]int, len(nodes))
	errchan := make(chan error, len(nodes))
	for _, n := range nodes {
		go func(nodeId crypto.Hash) {
			total, invalid, err := s.validateSnapshotEntriesForNode(nodeId, depth)
			if err != nil {
				logger.Printf("SNAPSHOT VALIDATION ERROR FOR NODE %s %s\n", nodeId, err.Error())
				errchan <- err
			}
			stats <- [2]int{total, invalid}
		}(n.IdForNetwork(networkId))
	}
	var total, invalid int
	for i := 0; i < len(nodes); i++ {
		select {
		case stat := <-stats:
			total += stat[0]
			invalid += stat[1]
		case err := <-errchan:
			return total, invalid, err
		}
	}
	return total, invalid, nil
}

func (s *BadgerStore) validateSnapshotEntriesForNode(nodeId crypto.Hash, depth uint64) (int, int, error) {
	logger.Printf("SNAPSHOT VALIDATE NODE %s BEGIN\n", nodeId)
	txn := s.snapshotsDB.NewTransaction(false)
	defer func() {
		txn.Discard()
		logger.Printf("SNAPSHOT VALIDATE NODE %s DONE\n", nodeId)
	}()

	head, err := readRound(txn, nodeId)
	if err != nil {
		return 0, 0, err
	}
	if head == nil {
		logger.Printf("SNAPSHOT VALIDATE NODE %s 0 ROUND\n", nodeId)
		return 0, 0, nil
	}

	logger.Printf("SNAPSHOT VALIDATE NODE %s %d ROUNDS\n", nodeId, head.Number)
	start := head.Number - depth
	if head.Number < depth {
		start = 0
	}
	invalid, total := 0, 0
	for i := start; i < head.Number; i++ {
		snapshots, err := readSnapshotsForNodeRound(txn, nodeId, i)
		if err != nil {
			return total, invalid, err
		}
		for _, s := range snapshots {
			total += 1
			item, err := txn.Get(graphTransactionKey(s.Transaction))
			if err != nil {
				return total, invalid, err
			}
			val, err := item.ValueCopy(nil)
			if err != nil {
				return total, invalid, err
			}
			ver, err := common.DecompressUnmarshalVersionedTransaction(val)
			if err != nil {
				return total, invalid, err
			}
			if s.Transaction.String() != ver.PayloadHash().String() {
				logger.Printf("MALFORMED TRANSACTION %s %s %#v\n", s.Transaction, ver.PayloadHash(), ver)
				invalid += 1
			}
			item, err = txn.Get(graphFinalizationKey(s.Transaction))
			if err != nil {
				return total, invalid, err
			}
			val, err = item.ValueCopy(nil)
			if err != nil {
				return total, invalid, err
			}
			if s.Hash.String() != hex.EncodeToString(val) {
				logger.Printf("DUPLICATED FINALIZATION %s %s\n", s.Hash, hex.EncodeToString(val))
			}
			dup, _ := crypto.HashFromString(hex.EncodeToString(val))
			topo, err := readSnapshotWithTopo(txn, dup)
			if err != nil {
				return total, invalid, err
			}
			if topo.Transaction.String() != s.Transaction.String() {
				logger.Printf("MALFORMED FINALIZATION %s %s\n", s.Hash, topo.Hash)
				invalid += 1
			}
		}
		_, _, hash := computeRoundHash(nodeId, i, snapshots)
		round, err := readRound(txn, hash)
		if err != nil {
			return total, invalid, err
		}
		if round == nil {
			logger.Printf("MISSING ROUND %s %d %s\n", nodeId, i, hash)
			invalid += 1
		} else if round.NodeId != nodeId || round.Number != i {
			logger.Printf("MALFORMED ROUND %s %d %s %s %d\n", nodeId, i, hash, round.NodeId, round.Number)
			invalid += 1
		}
	}
	return total, invalid, nil
}

func computeRoundHash(nodeId crypto.Hash, number uint64, snapshots []*common.SnapshotWithTopologicalOrder) (uint64, uint64, crypto.Hash) {
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].Timestamp < snapshots[j].Timestamp {
			return true
		}
		if snapshots[i].Timestamp > snapshots[j].Timestamp {
			return false
		}
		a, b := snapshots[i].Hash, snapshots[j].Hash
		return bytes.Compare(a[:], b[:]) < 0
	})
	start := snapshots[0].Timestamp
	end := snapshots[len(snapshots)-1].Timestamp
	if end >= start+config.SnapshotRoundGap {
		err := fmt.Errorf("ComputeRoundHash(%s, %d) %d %d %d", nodeId, number, start, end, start+config.SnapshotRoundGap)
		panic(err)
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, number)
	hash := crypto.NewHash(append(nodeId[:], buf...))
	for _, s := range snapshots {
		if s.Timestamp > end {
			panic(nodeId)
		}
		hash = crypto.NewHash(append(hash[:], s.Hash[:]...))
	}
	return start, end, hash
}
