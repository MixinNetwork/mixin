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
	return binary.BigEndian.Uint64(ival[l:])
}

func nodePledgeKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixNodePledge), publicSpend[:]...)
}

func nodeAcceptKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixNodeAccept), publicSpend[:]...)
}

func nodeDepartKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixNodeDepart), publicSpend[:]...)
}
