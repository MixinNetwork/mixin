package storage

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"runtime"
	"sort"
	"sync/atomic"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/pb"
)

func (s *BadgerStore) ValidateGraphEntries(networkId crypto.Hash) (int, int, error) {
	invalid, err := s.validateSnapshotEntries(networkId)
	if err != nil || invalid > 0 {
		return 0, invalid, err
	}
	return s.validateTransactionEntries()
}

func (s *BadgerStore) validateSnapshotEntries(networkId crypto.Hash) (int, error) {
	nodes := s.readAllNodes()
	stats := make(chan int, len(nodes))
	for _, n := range nodes {
		go func(nodeId crypto.Hash) {
			invalid, err := s.validateSnapshotEntriesForNode(nodeId)
			if err != nil {
				logger.Printf("SNAPSHOT VALIDATION ERROR FOR NODE %s %s\n", nodeId, err.Error())
			}
			stats <- invalid
		}(n.Signer.Hash().ForNetwork(networkId))
	}
	var invalid int
	for i := 0; i < len(nodes); i++ {
		invalid = invalid + <-stats
	}
	return invalid, nil
}

func (s *BadgerStore) validateSnapshotEntriesForNode(nodeId crypto.Hash) (int, error) {
	logger.Printf("SNAPSHOT VALIDATE NODE %s BEGIN\n", nodeId)
	txn := s.snapshotsDB.NewTransaction(false)
	defer func() {
		txn.Discard()
		logger.Printf("SNAPSHOT VALIDATE NODE %s DONE\n", nodeId)
	}()

	head, err := readRound(txn, nodeId)
	if err != nil {
		return 0, err
	}
	if head == nil {
		logger.Printf("SNAPSHOT VALIDATE NODE %s 0 ROUND\n", nodeId)
		return 0, nil
	}

	logger.Printf("SNAPSHOT VALIDATE NODE %s %d ROUNDS\n", nodeId, head.Number)
	var invalid int
	for i := uint64(0); i < head.Number; i++ {
		snapshots, err := readSnapshotsForNodeRound(txn, nodeId, i)
		if err != nil {
			return invalid, err
		}
		_, _, hash := computeRoundHash(nodeId, i, snapshots)
		round, err := readRound(txn, hash)
		if err != nil {
			return invalid, err
		}
		if round == nil {
			logger.Printf("MISSING ROUND %s %d %s\n", nodeId, i, hash)
			invalid += 1
		} else if round.NodeId != nodeId || round.Number != i {
			logger.Printf("MALFORMED ROUND %s %d %s %s %d\n", nodeId, i, hash, round.NodeId, round.Number)
			invalid += 1
		}
	}
	return invalid, nil
}

func (s *BadgerStore) validateTransactionEntries() (int, int, error) {
	var total, invalid int64
	stream := s.snapshotsDB.NewStream()
	stream.NumGo = runtime.NumCPU()
	stream.Prefix = []byte(graphPrefixTransaction)
	stream.LogPrefix = "Badger.ValidateGraphEntries"
	stream.ChooseKey = func(item *badger.Item) bool {
		atomic.AddInt64(&total, 1)
		err := item.Value(func(val []byte) error {
			ver, err := common.DecompressUnmarshalVersionedTransaction(val)
			if err != nil {
				return err
			}
			key := item.Key()
			var hash crypto.Hash
			copy(hash[:], key[len(graphPrefixTransaction):])
			if hash.String() != ver.PayloadHash().String() {
				atomic.AddInt64(&invalid, 1)
				logger.Printf("MALFORMED TRANSACTION %s %s %#v\n", hash.String(), ver.PayloadHash().String(), ver)
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
		return false
	}
	stream.KeyToList = nil
	stream.Send = func(list *pb.KVList) error { return nil }
	err := stream.Orchestrate(context.Background())

	return int(total), int(invalid), err
}

func (s *BadgerStore) readAllNodes() []*common.Node {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	nodes := s.ReadConsensusNodes()
	removed := readNodesInState(txn, graphPrefixNodeRemove)
	return append(nodes, removed...)
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
