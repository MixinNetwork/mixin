package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

const (
	graphPrefixGhost        = "GHOST"        // each output key should only be used once
	graphPrefixUTXO         = "UTXO"         // unspent outputs, including first consumed transaction hash
	graphPrefixTransaction  = "TRANSACTION"  // raw transaction, may not be finalized yet, if finalized with first finalized snapshot hash
	graphPrefixFinalization = "FINALIZATION" // transaction finalization hack
	graphPrefixUnique       = "UNIQUE"       // unique transaction in one node
	graphPrefixRound        = "ROUND"        // hash|node-if-cache {node:hash,number:734,references:{self-parent-round-hash,external-round-hash}}
	graphPrefixSnapshot     = "SNAPSHOT"     // {
	graphPrefixLink         = "LINK"         // self-external number
	graphPrefixNode         = "NODE"         // {head}
	graphPrefixTopology     = "TOPOLOGY"
)

func (s *BadgerStore) ReadSnapshotsForNodeRound(nodeId crypto.Hash, round uint64) ([]*common.Snapshot, error) {
	snapshots := make([]*common.Snapshot, 0)

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	key := graphSnapshotKey(nodeId, round, crypto.Hash{})
	prefix := key[:len(key)-len(crypto.Hash{})]
	for it.Seek(key); it.ValidForPrefix(prefix); it.Next() {
		item := it.Item()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return snapshots, err
		}
		var s common.Snapshot
		err = msgpack.Unmarshal(v, &s)
		if err != nil {
			return snapshots, err
		}
		snapshots = append(snapshots, &s)
	}

	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Timestamp < snapshots[j].Timestamp })
	return snapshots, nil
}

func (s *BadgerStore) ReadTransaction(hash crypto.Hash) (*common.Transaction, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	return readTransaction(txn, hash)
}

func (s *BadgerStore) WriteTransaction(tx *common.Transaction) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	// FIXME assert kind checks, not needed at all
	if config.Debug {
		txHash := tx.PayloadHash()
		for _, in := range tx.Inputs {
			if len(in.Genesis) > 0 {
				continue
			}

			if in.Deposit != nil {
				ival, err := readDepositInput(txn, in.Deposit)
				if err != nil {
					panic(fmt.Errorf("deposit check error %s", err.Error()))
				}
				if bytes.Compare(ival, txHash[:]) != 0 {
					panic(fmt.Errorf("deposit locked for transaction %s", hex.EncodeToString(ival)))
				}
				continue
			}

			key := utxoKey(in.Hash, in.Index)
			item, err := txn.Get([]byte(key))
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s", err.Error()))
			}
			ival, err := item.ValueCopy(nil)
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s", err.Error()))
			}
			var out common.UTXOWithLock
			err = msgpack.Unmarshal(ival, &out)
			if err != nil {
				panic(fmt.Errorf("UTXO check error %s", err.Error()))
			}
			if out.LockHash != txHash {
				panic(fmt.Errorf("utxo locked for transaction %s", out.LockHash))
			}
		}
	}
	// assert end

	err := writeTransaction(txn, tx)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) ReadRound(hash crypto.Hash) (*common.Round, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	return readRound(txn, hash)
}

func (s *BadgerStore) StartNewRound(node crypto.Hash, number, start uint64, references [2]crypto.Hash) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	self, err := readRound(txn, node)
	if err != nil {
		return err
	}
	external, err := readRound(txn, references[1])
	if err != nil {
		return err
	}

	// FIXME assert only, remove in future
	if config.Debug {
		if self == nil || self.Number != number-1 {
			panic("self final assert error")
		}
		if external == nil {
			panic("external final not exist")
		}
		old, err := readRound(txn, references[0])
		if err != nil {
			return err
		}
		if old != nil {
			panic("self final already exist")
		}
		link, err := readLink(txn, node, external.NodeId)
		if err != nil {
			return err
		}
		if link > external.Number {
			panic("external link backward")
		}
	}
	// assert end

	err = writeLink(txn, node, external.NodeId, external.Number)
	if err != nil {
		return err
	}
	err = writeRound(txn, references[0], self)
	if err != nil {
		return err
	}
	err = writeRound(txn, node, &common.Round{
		NodeId:     node,
		Number:     number,
		Timestamp:  start,
		References: references,
	})
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (s *BadgerStore) CheckTransactionFinalization(hash crypto.Hash) (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := graphFinalizationKey(hash)
	_, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *BadgerStore) CheckTransactionInNode(nodeId, hash crypto.Hash) (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := graphUniqueKey(nodeId, hash)
	_, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (s *BadgerStore) WriteSnapshot(snap *common.SnapshotWithTopologicalOrder) error {
	txn := s.snapshotsDB.NewTransaction(true)
	defer txn.Discard()

	// FIXME assert only, remove in future
	if config.Debug {
		cache, err := readRound(txn, snap.NodeId)
		if err != nil {
			return err
		}
		if cache == nil || snap.RoundNumber != cache.Number {
			panic("snapshot round number assert error")
		}
		if snap.References[0] != cache.References[0] || snap.References[1] != cache.References[1] {
			panic("snapshot references assert error")
		}
		tx, err := readTransaction(txn, snap.Transaction.PayloadHash())
		if err != nil {
			return err
		}
		if tx == nil {
			panic("snapshot transaction not exist")
		}
		key := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.Transaction.PayloadHash())
		_, err = txn.Get(key)
		if err == nil {
			panic("snapshot duplication")
		} else if err != badger.ErrKeyNotFound {
			return err
		}
	}
	// end assert

	err := finalizeTransaction(txn, &snap.Transaction.Transaction)
	if err != nil {
		return err
	}

	key := graphSnapshotKey(snap.NodeId, snap.RoundNumber, snap.Transaction.PayloadHash())
	val := common.MsgpackMarshalPanic(snap)
	err = txn.Set(key, val)
	if err != nil {
		return err
	}

	key = graphUniqueKey(snap.NodeId, snap.Transaction.PayloadHash())
	err = txn.Set(key, []byte{})
	if err != nil {
		return err
	}

	err = writeTopology(txn, snap)
	if err != nil {
		return err
	}
	return txn.Commit()
}

func (s *BadgerStore) ReadSnapshotsSinceTopology(topologyOffset, count uint64) ([]*common.SnapshotWithTopologicalOrder, error) {
	snapshots := make([]*common.SnapshotWithTopologicalOrder, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Seek(graphTopologyKey(topologyOffset))
	for ; it.ValidForPrefix([]byte(graphPrefixTopology)) && uint64(len(snapshots)) < count; it.Next() {
		item := it.Item()
		v, err := item.ValueCopy(nil)
		if err != nil {
			return snapshots, err
		}
		var snap common.SnapshotWithTopologicalOrder
		err = msgpack.Unmarshal(v, &snap)
		if err != nil {
			return snapshots, err
		}
		snap.Transaction.Hash = snap.Transaction.PayloadHash()
		snap.TopologicalOrder = topologyOrder(item.Key())
		snap.Hash = snap.PayloadHash()
		snapshots = append(snapshots, &snap)
	}

	return snapshots, nil
}

func (s *BadgerStore) TopologySequence() uint64 {
	var sequence uint64

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.PrefetchValues = false
	opts.Reverse = true

	it := txn.NewIterator(opts)
	defer it.Close()

	it.Seek(topologyKey(^uint64(0)))
	if it.ValidForPrefix([]byte(graphPrefixTopology)) {
		item := it.Item()
		sequence = topologyOrder(item.Key()) + 1
	}
	return sequence
}

func writeTopology(txn *badger.Txn, snap *common.SnapshotWithTopologicalOrder) error {
	key := graphTopologyKey(snap.TopologicalOrder)
	val := snap.PayloadHash()
	return txn.Set(key, val[:])
}

func readTransaction(txn *badger.Txn, hash crypto.Hash) (*common.Transaction, error) {
	var out common.Transaction
	key := graphTransactionKey(hash)
	err := graphReadValue(txn, key, &out)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return &out, err
}

func writeTransaction(txn *badger.Txn, tx *common.Transaction) error {
	key := graphTransactionKey(tx.PayloadHash())

	// FIXME assert only, remove in future
	if config.Debug {
		_, err := txn.Get(key)
		if err == nil {
			panic("transaction duplication")
		} else if err != badger.ErrKeyNotFound {
			return err
		}
	}
	// end assert

	val := common.MsgpackMarshalPanic(tx)
	return txn.Set(key, val)
}

func finalizeTransaction(txn *badger.Txn, tx *common.Transaction) error {
	key := graphFinalizationKey(tx.PayloadHash())
	_, err := txn.Get(key)
	if err == nil {
		return nil
	} else if err != badger.ErrKeyNotFound {
		return err
	}
	err = txn.Set(key, []byte{})
	if err != nil {
		return err
	}

	for _, utxo := range tx.UnspentOutputs() {
		err := writeUTXO(txn, utxo, tx.Extra)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeUTXO(txn *badger.Txn, utxo *common.UTXO, extra []byte) error {
	for _, k := range utxo.Keys {
		key := ghostKey(k)

		// FIXME assert kind checks, not needed at all
		if config.Debug {
			_, err := txn.Get(key)
			if err == nil {
				panic("ErrorValidateFailed")
			} else if err != badger.ErrKeyNotFound {
				return err
			}
		}
		// assert end

		err := txn.Set(key, []byte{0})
		if err != nil {
			return err
		}
	}
	key := utxoKey(utxo.Hash, utxo.Index)
	val := common.MsgpackMarshalPanic(utxo)
	err := txn.Set(key, val)
	if err != nil {
		return err
	}

	switch utxo.Type {
	case common.OutputTypeNodePledge:
		var publicSpend crypto.Key
		copy(publicSpend[:], extra)
		return writeNodePledge(txn, publicSpend, utxo.Hash)
	case common.OutputTypeNodeAccept:
		var publicSpend crypto.Key
		copy(publicSpend[:], extra)
		return writeNodeAccept(txn, publicSpend, utxo.Hash, false)
	case common.OutputTypeDomainAccept:
		var publicSpend crypto.Key
		copy(publicSpend[:], extra)
		return writeDomainAccept(txn, publicSpend, utxo.Hash)
	}

	return nil
}

func readLink(txn *badger.Txn, from, to crypto.Hash) (uint64, error) {
	key := graphLinkKey(from, to)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(ival), nil
}

func writeLink(txn *badger.Txn, from, to crypto.Hash, link uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, link)
	key := graphLinkKey(from, to)
	return txn.Set(key, buf)
}

func readRound(txn *badger.Txn, hash crypto.Hash) (*common.Round, error) {
	var out common.Round
	key := graphRoundKey(hash)
	err := graphReadValue(txn, key, &out)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	return &out, err
}

func writeRound(txn *badger.Txn, hash crypto.Hash, round *common.Round) error {
	key := graphRoundKey(hash)
	val := common.MsgpackMarshalPanic(round)
	return txn.Set(key, val)
}

func graphReadValue(txn *badger.Txn, key []byte, val interface{}) error {
	item, err := txn.Get(key)
	if err != nil {
		return err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return err
	}
	return msgpack.Unmarshal(ival, &val)
}

func graphRoundKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixRound), hash[:]...)
}

func graphTransactionKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixTransaction), hash[:]...)
}

func graphFinalizationKey(hash crypto.Hash) []byte {
	return append([]byte(graphPrefixFinalization), hash[:]...)
}

func graphUniqueKey(nodeId, hash crypto.Hash) []byte {
	key := append(hash[:], nodeId[:]...)
	return append([]byte(graphPrefixUnique), key...)
}

func graphSnapshotKey(nodeId crypto.Hash, round uint64, hash crypto.Hash) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, round)
	key := append([]byte(graphPrefixSnapshot), nodeId[:]...)
	key = append(key, buf...)
	return append(key, hash[:]...)
}

func graphLinkKey(from, to crypto.Hash) []byte {
	link := crypto.NewHash(append(from[:], to[:]...))
	return append([]byte(graphPrefixLink), link[:]...)
}

func graphTopologyKey(order uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, order)
	return append([]byte(graphPrefixTopology), buf...)
}

func graphTopologyOrder(key []byte) uint64 {
	order := key[len(graphPrefixTopology):]
	return binary.BigEndian.Uint64(order)
}
