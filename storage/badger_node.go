package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

const (
	graphPrefixNodePledge = "NODESTATEPLEDGE"
	graphPrefixNodeAccept = "NODESTATEACCEPT"
	graphPrefixNodeDepart = "NODESTATEDEPART"
	graphPrefixNodeRemove = "NODESTATEREMOVE"
	graphPrefixNodeCancel = "NODESTATECANCEL"

	graphPrefixNodeOperation = "NODEOPERATION"
)

func (s *BadgerStore) ReadConsensusNodes() []*common.Node {
	nodes := make([]*common.Node, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	accepted := readNodesInState(txn, graphPrefixNodeAccept)
	for _, n := range accepted {
		n.State = common.NodeStateAccepted
		nodes = append(nodes, n)
	}
	pledging := readNodesInState(txn, graphPrefixNodePledge)
	for _, n := range pledging {
		n.State = common.NodeStatePledging
		nodes = append(nodes, n)
	}
	departing := readNodesInState(txn, graphPrefixNodeDepart)
	for _, n := range departing {
		n.State = common.NodeStateDeparting
		nodes = append(nodes, n)
	}
	return nodes
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
		return fmt.Errorf("invalid operation lock %s %s %d", lastTx, lastOp, lastTs)
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
		order := item.Key()[len(graphPrefixNodeOperation):]
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

func readNodesInState(txn *badger.Txn, nodeState string) []*common.Node {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(nodeState)
	nodes := make([]*common.Node, 0)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		signer := nodeSignerForState(item.Key(), nodeState)
		ival, err := item.ValueCopy(nil)
		if err != nil {
			panic(err)
		}
		nodes = append(nodes, &common.Node{
			Signer:      signer,
			Payee:       nodePayee(ival),
			Transaction: nodeTransaction(ival),
			Timestamp:   nodeTimestamp(ival),
		})
	}
	return nodes
}

func writeNodeCancel(txn *badger.Txn, signer, payee crypto.Key, tx crypto.Hash, timestamp uint64) error {
	// TODO these checks are only assert kind checks, not needed at all
	key := nodePledgeKey(signer)
	_, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return fmt.Errorf("node not pledging yet %s", signer.String())
	} else if err != nil {
		return err
	}

	err = txn.Delete(key)
	if err != nil {
		return err
	}
	key = nodeCancelKey(signer)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timestamp)
	val := append(payee[:], tx[:]...)
	val = append(val, buf...)
	return txn.Set(key, val)
}

func writeNodeAccept(txn *badger.Txn, signer, payee crypto.Key, tx crypto.Hash, timestamp uint64, genesis bool) error {
	// TODO these checks are only assert kind checks, not needed at all
	key := nodePledgeKey(signer)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		if !genesis {
			return fmt.Errorf("node not pledging yet %s", signer.String())
		}
	} else if err != nil {
		return err
	}
	if !genesis {
		ival, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		if bytes.Compare(payee[:], ival[:len(payee)]) != 0 {
			return fmt.Errorf("node not accept to the same payee account %s %s", hex.EncodeToString(ival[:len(payee)]), payee.String())
		}
	}

	err = txn.Delete(key)
	if err != nil {
		return err
	}
	key = nodeAcceptKey(signer)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timestamp)
	val := append(payee[:], tx[:]...)
	val = append(val, buf...)
	return txn.Set(key, val)
}

func writeNodePledge(txn *badger.Txn, signer, payee crypto.Key, tx crypto.Hash, timestamp uint64) error {
	// TODO these checks are only assert kind checks, not needed at all
	key := nodeAcceptKey(signer)
	_, err := txn.Get(key)
	if err == nil {
		return fmt.Errorf("node already accepted %s", signer.String())
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	pledging := readNodesInState(txn, graphPrefixNodePledge)
	if len(pledging) > 0 {
		node := pledging[0]
		return fmt.Errorf("node %s is pledging", node.Signer.PublicSpendKey.String())
	}

	departing := readNodesInState(txn, graphPrefixNodeDepart)
	if len(departing) > 0 {
		node := departing[0]
		return fmt.Errorf("node %s is departing", node.Signer.PublicSpendKey.String())
	}

	key = nodePledgeKey(signer)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timestamp)
	val := append(payee[:], tx[:]...)
	val = append(val, buf...)
	return txn.Set(key, val)
}

func nodeSignerForState(key []byte, nodeState string) common.Address {
	var publicSpend crypto.Key
	copy(publicSpend[:], key[len(nodeState):])
	privateView := publicSpend.DeterministicHashDerive()
	return common.Address{
		PrivateViewKey: privateView,
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
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

func nodeTimestamp(ival []byte) uint64 {
	l := len(crypto.Key{}) + len(crypto.Hash{})
	if len(ival) == l+8 {
		return binary.BigEndian.Uint64(ival[l:])
	}
	return 0
}

func nodePledgeKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixNodePledge), publicSpend[:]...)
}

func nodeCancelKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixNodeCancel), publicSpend[:]...)
}

func nodeAcceptKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixNodeAccept), publicSpend[:]...)
}

func nodeDepartKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixNodeDepart), publicSpend[:]...)
}

func nodeOperationKey(timestamp uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timestamp)
	return append([]byte(graphPrefixNodeOperation), buf...)
}
