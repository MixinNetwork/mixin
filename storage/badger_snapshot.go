package storage

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
	"github.com/vmihailenco/msgpack"
)

const (
	snapshotsPrefixSnapshot    = "SNAPSHOT"    // transaction hash to snapshot meta, mainly node and consensus timestamp
	snapshotsPrefixTransaction = "TRANSACTION" // transaction hash to snapshot meta, mainly node and consensus timestamp
	snapshotsPrefixGraph       = "GRAPH"       // consensus directed asyclic graph data store
	snapshotsPrefixUTXO        = "UTXO"        // unspent outputs, will be deleted once consumed
	snapshotsPrefixDeposit     = "DEPOSIT"     // unspent outputs, will be deleted once consumed
	snapshotsPrefixNodeRound   = "NODEROUND"   // node specific info, e.g. round number, round hash
	snapshotsPrefixNodeLink    = "NODELINK"    // latest node round links
	snapshotsPrefixGhost       = "GHOST"       // each output key should only be used once
)

func (s *BadgerStore) SnapshotsReadSnapshotsForNodeRound(nodeIdWithNetwork crypto.Hash, round uint64) ([]*common.Snapshot, error) {
	snapshots := make([]*common.Snapshot, 0)

	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	key := graphKey(nodeIdWithNetwork, round, 0)
	prefix := key[:len(key)-8]
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

	return snapshots, nil
}

func (s *BadgerStore) SnapshotsReadSnapshotByTransactionHash(hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	return readSnapshotByTransactionHash(txn, hash)
}

func (s *BadgerStore) SnapshotsWriteSnapshot(snapshot *common.SnapshotWithTopologicalOrder) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		return writeSnapshot(txn, snapshot, false)
	})
}

func (s *BadgerStore) SnapshotsReadUTXO(hash crypto.Hash, index int) (*common.UTXO, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	key := utxoKey(hash, index)
	item, err := txn.Get(key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}

	var out common.UTXO
	err = msgpack.Unmarshal(ival, &out)
	return &out, err
}

func readDepositInput(txn *badger.Txn, deposit *common.DepositData) ([]byte, error) {
	key := depositKey(deposit)
	item, err := txn.Get(key)
	if err != nil {
		return nil, err
	}
	return item.ValueCopy(nil)
}

func (s *BadgerStore) SnapshotsCheckDepositInput(deposit *common.DepositData, tx crypto.Hash) error {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	ival, err := readDepositInput(txn, deposit)
	if err == badger.ErrKeyNotFound {
		return nil
	} else if err != nil {
		return err
	}
	if bytes.Compare(ival, tx[:]) == 0 {
		return nil
	}
	return fmt.Errorf("invalid lock %s %s", hex.EncodeToString(ival), hex.EncodeToString(tx[:]))
}

func (s *BadgerStore) SnapshotsLockDepositInput(deposit *common.DepositData, tx crypto.Hash) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		key := depositKey(deposit)
		ival, err := readDepositInput(txn, deposit)
		save := func() error {
			return txn.Set(key, tx[:])
		}
		if err == badger.ErrKeyNotFound {
			return save()
		}
		if err != nil {
			return err
		}
		if bytes.Compare(ival, tx[:]) != 0 {
			return fmt.Errorf("deposit locked for transaction %s", hex.EncodeToString(ival))
		}
		return save()
	})
}

func (s *BadgerStore) SnapshotsLockUTXO(hash crypto.Hash, index int, tx crypto.Hash) (*common.UTXO, error) {
	var utxo *common.UTXO
	err := s.snapshotsDB.Update(func(txn *badger.Txn) error {
		key := utxoKey(hash, index)
		item, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		ival, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		var out common.UTXOWithLock
		err = msgpack.Unmarshal(ival, &out)
		if err != nil {
			return err
		}

		if out.LockHash.HasValue() && out.LockHash != tx {
			return fmt.Errorf("utxo locked for transaction %s", out.LockHash)
		}
		out.LockHash = tx
		err = txn.Set([]byte(key), common.MsgpackMarshalPanic(out))
		utxo = &out.UTXO
		return err
	})
	return utxo, err
}

func (s *BadgerStore) SnapshotsCheckGhost(key crypto.Key) (bool, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	_, err := txn.Get([]byte(ghostKey(key)))
	if err == badger.ErrKeyNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *BadgerStore) SnapshotsLoadGenesis(snapshots []*common.SnapshotWithTopologicalOrder) error {
	return s.snapshotsDB.Update(func(txn *badger.Txn) error {
		if checkGenesisLoad(txn) {
			return nil
		}

		filter := make(map[crypto.Hash]bool)
		for _, snap := range snapshots {
			if !filter[snap.NodeId] {
				filter[snap.NodeId] = true
				err := writeRoundMeta(txn, snap.NodeId, snap.RoundNumber, snap.Timestamp)
				if err != nil {
					return err
				}
			}
			err := writeSnapshot(txn, snap, true)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func checkGenesisLoad(txn *badger.Txn) bool {
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Rewind()
	return it.Valid()
}

func readSnapshotByTransactionHash(txn *badger.Txn, hash crypto.Hash) (*common.SnapshotWithTopologicalOrder, error) {
	item, err := txn.Get(snapshotKey(hash))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	meta, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}

	key := meta[:len(graphKey(crypto.Hash{}, 0, 0))]
	topo := binary.BigEndian.Uint64(meta[len(key):])
	item, err = txn.Get(key)
	if err == badger.ErrKeyNotFound {
		panic(hash.String())
	} else if err != nil {
		return nil, err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	var s common.SnapshotWithTopologicalOrder
	err = msgpack.Unmarshal(val, &s)
	s.Transaction.Hash = s.Transaction.PayloadHash()
	s.TopologicalOrder = topo
	s.Hash = s.PayloadHash()
	return &s, err
}

func pruneSnapshot(txn *badger.Txn, tx crypto.Hash) error {
	return nil
}

func writeSnapshot(txn *badger.Txn, snapshot *common.SnapshotWithTopologicalOrder, genesis bool) error {
	txHash := snapshot.Transaction.PayloadHash()
	// FIXME what if same transaction but different snapshot hash
	_, err := txn.Get(snapshotKey(txHash))
	if err == nil {
		return nil
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	roundMeta, err := readRoundMeta(txn, snapshot.NodeId)
	if err != nil {
		return err
	}
	roundNumber, roundStart := roundMeta[0], roundMeta[1]

	// TODO this section is only an assert kind check, not needed at all
	if snapshot.RoundNumber < roundNumber || snapshot.RoundNumber > roundNumber+1 {
		panic(fmt.Errorf("snapshot round error %d %d %d %d", roundNumber, roundStart, snapshot.RoundNumber, snapshot.Timestamp))
	}
	if snapshot.RoundNumber == roundNumber && snapshot.Timestamp >= config.SnapshotRoundGap+roundStart {
		panic(fmt.Errorf("snapshot old round timestamp error %d %d %d %d", roundNumber, roundStart, snapshot.RoundNumber, snapshot.Timestamp))
	}
	if snapshot.RoundNumber == roundNumber+1 && snapshot.Timestamp < config.SnapshotRoundGap+roundStart {
		panic(fmt.Errorf("snapshot new round timestamp error %d %d %d %d", roundNumber, roundStart, snapshot.RoundNumber, snapshot.Timestamp))
	}

	// FIXME should ensure round meta and snapshot consistence, how to move out here?
	if snapshot.RoundNumber == roundNumber+1 || snapshot.Timestamp < roundStart {
		err = writeRoundMeta(txn, snapshot.NodeId, snapshot.RoundNumber, snapshot.Timestamp)
		if err != nil {
			return err
		}
	}

	// FIXME should ensure round links and snapshot consistence, how to move out here?
	for to, link := range snapshot.RoundLinks {
		err = writeRoundLink(txn, snapshot.NodeId, to, link)
		if err != nil {
			return err
		}
	}

	// FIXME assert kind checks, not needed at all
	for _, in := range snapshot.Transaction.Inputs {
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

	for _, utxo := range snapshot.UnspentOutputs() {
		for _, k := range utxo.Keys {
			key := ghostKey(k)

			_, err := txn.Get(key) // TODO this check is only an assert kind check, not needed at all
			if err == nil {
				panic("ErrorValidateFailed")
			} else if err != badger.ErrKeyNotFound {
				return err
			}

			err = txn.Set(key, []byte{0})
			if err != nil {
				return err
			}
		}
		key := utxoKey(utxo.Hash, utxo.Index)
		val := common.MsgpackMarshalPanic(utxo)
		err = txn.Set(key, val)
		if err != nil {
			return err
		}

		switch utxo.Type {
		case common.OutputTypeNodePledge:
			var publicSpend crypto.Key
			copy(publicSpend[:], snapshot.Transaction.Extra)
			err = writeNodePledge(txn, publicSpend, snapshot.Transaction.PayloadHash())
			if err != nil {
				return err
			}
		case common.OutputTypeNodeAccept:
			var publicSpend crypto.Key
			copy(publicSpend[:], snapshot.Transaction.Extra)
			err = writeNodeAccept(txn, publicSpend, snapshot.Transaction.PayloadHash(), genesis)
			if err != nil {
				return err
			}
		case common.OutputTypeDomainAccept:
			var publicSpend crypto.Key
			copy(publicSpend[:], snapshot.Transaction.Extra)
			err = writeDomainAccept(txn, publicSpend, snapshot.Transaction.PayloadHash())
			if err != nil {
				return err
			}
		}
	}

	key := graphKey(snapshot.NodeId, snapshot.RoundNumber, snapshot.Timestamp)

	_, err = txn.Get(key) // TODO this check is only an assert kind check, not needed at all
	if err == nil {
		panic("ErrorValidateFailed")
	} else if err != badger.ErrKeyNotFound {
		return err
	}

	val := common.MsgpackMarshalPanic(snapshot)
	err = txn.Set(key, val)
	if err != nil {
		return err
	}

	// not related to consensus
	seq := snapshot.TopologicalOrder
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, seq)
	meta := append(key, buf...)
	meta = append(meta, byte(len(snapshot.References)))
	for _, ref := range snapshot.References {
		meta = append(meta, ref[:]...)
	}
	err = txn.Set(snapshotKey(snapshot.Transaction.PayloadHash()), meta)
	if err != nil {
		return err
	}
	return writeSnapshotTopology(txn, snapshot)
}

func snapshotKey(transactionHash crypto.Hash) []byte {
	return append([]byte(snapshotsPrefixSnapshot), transactionHash[:]...)
}

func graphKey(nodeIdWithNetwork crypto.Hash, round, ts uint64) []byte {
	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf, round)
	binary.BigEndian.PutUint64(buf[8:], ts)
	key := append([]byte(snapshotsPrefixGraph), nodeIdWithNetwork[:]...)
	return append(key, buf...)
}

func utxoKey(hash crypto.Hash, index int) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	size := binary.PutVarint(buf, int64(index))
	key := append([]byte(snapshotsPrefixUTXO), hash[:]...)
	return append(key, buf[:size]...)
}

func depositKey(deposit *common.DepositData) []byte {
	hash := crypto.NewHash(common.MsgpackMarshalPanic(deposit))
	return append([]byte(snapshotsPrefixDeposit), hash[:]...)
}

func ghostKey(k crypto.Key) []byte {
	return append([]byte(snapshotsPrefixGhost), k[:]...)
}
