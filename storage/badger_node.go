package storage

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v3"
)

const (
	graphPrefixNodeStateQueue = "NODESTATEQUEUE"
	graphPrefixNodeOperation  = "NODEOPERATION"
)

func readAllNodes(txn *badger.Txn, threshold uint64, withState bool) []*common.Node {
	prefix := []byte(graphPrefixNodeStateQueue)
	opts := badger.DefaultIteratorOptions
	opts.PrefetchSize = 30
	opts.Prefix = prefix
	it := txn.NewIterator(opts)
	defer it.Close()

	nodes := make([]*common.Node, 0)
	for it.Seek(prefix); it.Valid(); it.Next() {
		item := it.Item()
		signer, ts := nodeSignerFromStateKey(item.KeyCopy(nil))
		ival, err := item.ValueCopy(nil)
		if err != nil {
			panic(err)
		}
		if ts == 0 {
			panic(fmt.Errorf("invalid node timestamp %s", signer.String()))
		}
		if ts > threshold {
			continue
		}
		n := &common.Node{
			Signer:      signer,
			Payee:       nodePayee(ival),
			Transaction: nodeTransaction(ival),
			State:       nodeState(ival),
			Timestamp:   ts,
		}
		nodes = append(nodes, n)
	}

	filter := make(map[crypto.Hash]*common.Node)
	for i, n := range nodes {
		filter[n.Signer.Hash()] = n
		if i == 0 {
			continue
		}
		if p := nodes[i-1]; n.Timestamp < p.Timestamp {
			panic(fmt.Errorf("malformed order %s:%d:%s %s:%d:%s", p.Signer, p.Timestamp, p.State, n.Signer, n.Timestamp, n.State))
		}
	}

	if withState {
		return nodes
	}
	nodes = make([]*common.Node, 0)
	for _, n := range filter {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Timestamp < nodes[j].Timestamp
	})
	return nodes
}

func (s *BadgerStore) ReadAllNodes(threshold uint64, withState bool) []*common.Node {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readAllNodes(txn, threshold, withState)
}

func (s *BadgerStore) AddNodeOperation(tx *common.VersionedTransaction, timestamp, threshold uint64) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	var op string
	switch tx.TransactionType() {
	case common.TransactionTypeNodePledge:
		op = "PLEDGE"
	case common.TransactionTypeNodeCancel:
		op = "CANCEL"
	}
	if op == "" {
		return fmt.Errorf("invalid operation %d %s", tx.TransactionType(), op)
	}
	hash := tx.PayloadHash()

	lastOp, lastTx, lastTs, err := readLastNodeOperation(txn)
	if err != nil {
		return err
	}

	if lastTs+threshold >= timestamp {
		if lastOp == op && lastTx == hash {
			return nil
		}
		if hash.String() == "12e3d4dbc8fe04888d080c6223f17e64886a7d8eb458704c74efb13cc6ce340f" {
			logger.Printf("FORK invalid operation lock %s %s %d\n", lastTx, lastOp, lastTs)
		} else {
			return fmt.Errorf("invalid operation lock %s %s %d", lastTx, lastOp, lastTs)
		}
	}

	val := append(hash[:], []byte(op)...)
	err = txn.Set(nodeOperationKey(timestamp), val)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func readLastNodeOperation(txn *badger.Txn) (string, crypto.Hash, uint64, error) {
	var timestamp uint64
	var hash crypto.Hash

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(nodeOperationKey(^uint64(0)))
	if it.ValidForPrefix([]byte(graphPrefixNodeOperation)) {
		item := it.Item()
		order := item.KeyCopy(nil)[len(graphPrefixNodeOperation):]
		timestamp = binary.BigEndian.Uint64(order)

		val, err := item.ValueCopy(nil)
		if err != nil {
			return "", hash, timestamp, err
		}
		copy(hash[:], val)
		return string(val[len(hash):]), hash, timestamp, nil
	}
	return "", hash, timestamp, nil
}

func writeNodeCancel(txn *badger.Txn, signer, payee crypto.Key, tx crypto.Hash, timestamp uint64) error {
	offset := timestamp + uint64(config.KernelNodeAcceptPeriodMinimum)
	nodes := readAllNodes(txn, offset, true)
	last := nodes[len(nodes)-1]
	if last.State != common.NodeStatePledging {
		return fmt.Errorf("node %s is %s@%d while tx %s", last.Signer, last.State, last.Timestamp, tx.String())
	}
	if last.Signer.PublicSpendKey != signer || last.Payee.PublicSpendKey != payee {
		return fmt.Errorf("node %s %s not match at pledging", last.Signer.PublicSpendKey, signer)
	}

	key := nodeStateQueueKey(signer, timestamp)
	val := nodeEntryValue(payee, tx, common.NodeStateCancelled)
	return txn.Set(key, val)
}

func writeNodeRemove(txn *badger.Txn, signer, payee crypto.Key, tx crypto.Hash, timestamp uint64) error {
	offset := timestamp + uint64(config.KernelNodeAcceptPeriodMinimum)
	nodes := readAllNodes(txn, offset, true)
	last := nodes[len(nodes)-1]
	switch last.State {
	case common.NodeStateAccepted:
	case common.NodeStateRemoved:
	case common.NodeStateCancelled:
	default:
		return fmt.Errorf("node %s is %s@%d while tx %s", last.Signer, last.State, last.Timestamp, tx.String())
	}

	var node *common.Node
	for _, n := range nodes {
		if n.Signer.PublicSpendKey == signer {
			node = n
		}
	}
	if node == nil {
		return fmt.Errorf("node not available to remove %s", signer)
	}
	if node.Payee.PublicSpendKey != payee {
		return fmt.Errorf("node %s %s not match at %s", last.Payee.PublicSpendKey, payee, node.State)
	}
	if node.State != common.NodeStateAccepted {
		return fmt.Errorf("node %s %s not match at %s", last.Payee.PublicSpendKey, payee, node.State)
	}

	key := nodeStateQueueKey(signer, timestamp)
	val := nodeEntryValue(payee, tx, common.NodeStateRemoved)
	return txn.Set(key, val)
}

func writeNodeAccept(txn *badger.Txn, signer, payee crypto.Key, tx crypto.Hash, timestamp uint64, genesis bool) error {
	if !genesis {
		offset := timestamp + uint64(config.KernelNodeAcceptPeriodMinimum)
		nodes := readAllNodes(txn, offset, true)
		last := nodes[len(nodes)-1]
		if last.State != common.NodeStatePledging {
			return fmt.Errorf("node %s is %s@%d while tx %s", last.Signer, last.State, last.Timestamp, tx.String())
		}
		if last.Signer.PublicSpendKey != signer || last.Payee.PublicSpendKey != payee {
			return fmt.Errorf("node %s %s not match at pledging", last.Signer.PublicSpendKey, signer)
		}
	}

	key := nodeStateQueueKey(signer, timestamp)
	val := nodeEntryValue(payee, tx, common.NodeStateAccepted)
	return txn.Set(key, val)
}

func writeNodePledge(txn *badger.Txn, signer, payee crypto.Key, tx crypto.Hash, timestamp uint64) error {
	offset := timestamp + uint64(config.KernelNodePledgePeriodMinimum)
	nodes := readAllNodes(txn, offset, false)
	for _, n := range nodes {
		switch n.State {
		case common.NodeStateAccepted:
		case common.NodeStateRemoved:
		case common.NodeStateCancelled:
		default:
			return fmt.Errorf("node %s is %s@%d while tx %s", n.Signer, n.State, n.Timestamp, tx.String())
		}
	}

	for _, n := range nodes {
		if n.Signer.PublicSpendKey == signer || n.Transaction == tx {
			return fmt.Errorf("node %s is already %s@%d", n.Signer, n.State, n.Timestamp)
		}
	}

	key := nodeStateQueueKey(signer, timestamp)
	val := nodeEntryValue(payee, tx, common.NodeStatePledging)
	return txn.Set(key, val)
}

func nodeStateQueueKey(signer crypto.Key, timestamp uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timestamp)
	key := append([]byte(graphPrefixNodeStateQueue), buf...)
	return append(key, signer[:]...)
}

func nodeEntryValue(payee crypto.Key, tx crypto.Hash, state string) []byte {
	val := append(payee[:], tx[:]...)
	return append(val, []byte(state)...)
}

func nodeSignerFromStateKey(key []byte) (common.Address, uint64) {
	var publicSpend crypto.Key
	key = key[len(graphPrefixNodeStateQueue):]
	ts := binary.BigEndian.Uint64(key[:8])
	copy(publicSpend[:], key[8:])
	privateView := publicSpend.DeterministicHashDerive()
	return common.Address{
		PrivateViewKey: privateView,
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}, ts
}

func nodePayee(ival []byte) common.Address {
	var publicSpend crypto.Key
	copy(publicSpend[:], ival[:len(publicSpend)])
	privateView := publicSpend.DeterministicHashDerive()
	return common.Address{
		PrivateViewKey: privateView,
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
}

func nodeTransaction(ival []byte) crypto.Hash {
	var tx crypto.Hash
	copy(tx[:], ival[len(crypto.Key{}):])
	return tx
}

func nodeState(ival []byte) string {
	l := len(crypto.Key{}) + len(crypto.Hash{})
	return string(ival[l:])
}

func nodeOperationKey(timestamp uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timestamp)
	return append([]byte(graphPrefixNodeOperation), buf...)
}
