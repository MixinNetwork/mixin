package storage

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v2"
)

// FIXME remove this legacy node state migration file

const (
	legacygraphPrefixNodePledge = "NODESTATEPLEDGE"
	legacygraphPrefixNodeAccept = "NODESTATEACCEPT"
	legacygraphPrefixNodeResign = "NODESTATERESIGN"
	legacygraphPrefixNodeRemove = "NODESTATEREMOVE"
	legacygraphPrefixNodeCancel = "NODESTATECANCEL"
)

func (s *BadgerStore) TryToMigrateNodeStateQueue() error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	lnodes := readAllLegacyNodes(txn)
	if len(lnodes) == 0 {
		return nil
	}
	sort.Slice(lnodes, func(i, j int) bool {
		a, b := lnodes[i], lnodes[j]
		if a.Timestamp < b.Timestamp {
			return true
		}
		if a.Timestamp > b.Timestamp {
			return false
		}
		return bytes.Compare(a.Signer.PublicSpendKey[:], b.Signer.PublicSpendKey[:]) < 0
	})

	nodes := readAllNodes(txn, uint64(time.Now().UnixNano()), true)
	if len(nodes) != 0 {
		return fmt.Errorf("malformed state with both legacy and new nodes %d %d", len(lnodes), len(nodes))
	}

	for _, n := range lnodes {
		key := nodeStateQueueKey(n.Signer.PublicSpendKey, n.Timestamp)
		val := nodeEntryValue(n.Payee.PublicSpendKey, n.Transaction, n.State)
		err := txn.Set(key, val)
		if err != nil {
			return err
		}
	}
	return txn.Commit()
}

func readAllLegacyNodes(txn *badger.Txn) []*common.Node {
	nodes := make([]*common.Node, 0)
	accepted := readLagacyNodesInState(txn, legacygraphPrefixNodeAccept)
	nodes = append(nodes, accepted...)
	pledging := readLagacyNodesInState(txn, legacygraphPrefixNodePledge)
	nodes = append(nodes, pledging...)
	resigning := readLagacyNodesInState(txn, legacygraphPrefixNodeResign)
	nodes = append(nodes, resigning...)
	removed := readLagacyNodesInState(txn, legacygraphPrefixNodeRemove)
	nodes = append(nodes, removed...)
	canceled := readLagacyNodesInState(txn, legacygraphPrefixNodeCancel)
	return append(nodes, canceled...)
}

func readLagacyNodesInState(txn *badger.Txn, nodeState string) []*common.Node {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(nodeState)
	nodes := make([]*common.Node, 0)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		ival, err := item.ValueCopy(nil)
		if err != nil {
			panic(err)
		}
		var nc []*common.Node
		tx := nodeTransaction(ival)
		switch nodeState {
		case legacygraphPrefixNodePledge:
			nc = readLegacyPledgeNodes(txn, tx)
		case legacygraphPrefixNodeAccept:
			nc = readLegacyAcceptNodes(txn, tx)
		case legacygraphPrefixNodeResign:
			panic("should not have these yet")
		case legacygraphPrefixNodeRemove:
			nc = readLegacyRemoveNodes(txn, tx)
		case legacygraphPrefixNodeCancel:
			nc = readLegacyCancelNodes(txn, tx)
		}
		nodes = append(nodes, nc...)
	}
	return nodes
}

var legacyNodeStateSnapshotMap = map[string]string{
	"txhash": "snaphash",
}

func readSnapshotFromTx(txn *badger.Txn, h crypto.Hash) (*common.VersionedTransaction, *common.SnapshotWithTopologicalOrder) {
	tx, final, err := readTransactionAndFinalization(txn, h)
	if err != nil {
		panic(err)
	}
	if final == "MISSING" {
		final = legacyNodeStateSnapshotMap[h.String()]
	}
	hash, err := crypto.HashFromString(final)
	if err != nil {
		panic(err)
	}
	snap, err := readSnapshotWithTopo(txn, hash)
	if err != nil {
		panic(err)
	}
	return tx, snap
}

func readLegacyPledgeNodes(txn *badger.Txn, h crypto.Hash) []*common.Node {
	tx, snap := readSnapshotFromTx(txn, h)
	logger.Printf(`"%s":"%s",\n`, tx.PayloadHash(), snap.PayloadHash())

	var signer, payee common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicViewKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicViewKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()

	node := &common.Node{
		Signer:      signer,
		Payee:       payee,
		State:       common.NodeStatePledging,
		Transaction: h,
		Timestamp:   snap.Timestamp,
	}
	return []*common.Node{node}
}

func readLegacyAcceptNodes(txn *badger.Txn, h crypto.Hash) []*common.Node {
	tx, snap := readSnapshotFromTx(txn, h)
	logger.Printf(`"%s":"%s",\n`, tx.PayloadHash(), snap.PayloadHash())

	var signer, payee common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicViewKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicViewKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()

	node := &common.Node{
		Signer:      signer,
		Payee:       payee,
		State:       common.NodeStateAccepted,
		Transaction: h,
		Timestamp:   snap.Timestamp,
	}
	nodes := []*common.Node{node}

	if len(tx.Inputs[0].Genesis) > 0 {
		return nodes
	}

	pn := readLegacyPledgeNodes(txn, tx.Inputs[0].Hash)
	return append(nodes, pn...)
}

func readLegacyCancelNodes(txn *badger.Txn, h crypto.Hash) []*common.Node {
	tx, snap := readSnapshotFromTx(txn, h)
	logger.Printf(`"%s":"%s",\n`, tx.PayloadHash(), snap.PayloadHash())

	var signer, payee common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicViewKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicViewKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()

	node := &common.Node{
		Signer:      signer,
		Payee:       payee,
		State:       common.NodeStatePledging,
		Transaction: h,
		Timestamp:   snap.Timestamp,
	}
	nodes := []*common.Node{node}

	pn := readLegacyPledgeNodes(txn, tx.Inputs[0].Hash)
	return append(nodes, pn...)
}

func readLegacyRemoveNodes(txn *badger.Txn, h crypto.Hash) []*common.Node {
	tx, snap := readSnapshotFromTx(txn, h)
	logger.Printf(`"%s":"%s",\n`, tx.PayloadHash(), snap.PayloadHash())

	var signer, payee common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	copy(payee.PublicSpendKey[:], tx.Extra[len(signer.PublicSpendKey):])
	signer.PrivateViewKey = signer.PublicViewKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	payee.PrivateViewKey = payee.PublicViewKey.DeterministicHashDerive()
	payee.PublicViewKey = payee.PrivateViewKey.Public()

	node := &common.Node{
		Signer:      signer,
		Payee:       payee,
		State:       common.NodeStatePledging,
		Transaction: h,
		Timestamp:   snap.Timestamp,
	}
	nodes := []*common.Node{node}

	an := readLegacyAcceptNodes(txn, tx.Inputs[0].Hash)
	return append(nodes, an...)
}
