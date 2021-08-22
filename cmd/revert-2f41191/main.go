package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v3"
)

func main() {
	dbDir := flag.String("db", "/tmp/mixin/snapshots", "the mixin badger snapshots directory")
	flag.Parse()

	db, err := openDB(*dbDir)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	revertSnapshot2f41191(db)
}

func revertSnapshot2f41191(db *badger.DB) {
	txn := db.NewTransaction(true)
	defer txn.Discard()

	node, err := hex.DecodeString("a721a4fc0c667c4a1222c8d80350cbe07dab55c49942c8100a8c5e2f5bb4ec50")
	if err != nil {
		panic(err)
	}
	snap, err := hex.DecodeString("2f41191db0e80a079497418f2dff328426f4a186d08f07f348423a2e952de5c8")
	if err != nil {
		panic(err)
	}
	signer, err := hex.DecodeString("979939097dd50d0d6be42c47b3235c07108c28ce7cca150eed3b745283a9ef96")
	if err != nil {
		panic(err)
	}
	payee, err := hex.DecodeString("39d749ab642df3e0e5573052b7031cd1e96c328e6f73d22851c475d96b7c5257")
	if err != nil {
		panic(err)
	}
	tx, err := hex.DecodeString("5427ccbdb99a7eadfe271be34afe3a8101e304be7a5d8a7e8be57d3990e7c270")
	if err != nil {
		panic(err)
	}
	pledgeTx, err := hex.DecodeString("9d87a3085035bba4b58bdad03ef61958f980e0377271721bd0cb3ff2d21b3f08")
	if err != nil {
		panic(err)
	}

	item, err := txn.Get(append([]byte("ROUND"), node...))
	if err != nil {
		panic(err)
	}
	ival, err := item.ValueCopy(nil)
	if err != nil {
		panic(err)
	}
	var round common.Round
	err = common.MsgpackUnmarshal(ival, &round)
	if err != nil {
		panic(err)
	}
	if round.Number > 198415 {
		panic(round.Number)
	}
	if round.Number < 198415 {
		fmt.Printf("round at %d\n", round.Number)
		return
	}

	item, err = txn.Get(graphSnapshotKey(node, round.Number, tx))
	if err == badger.ErrKeyNotFound {
		fmt.Println("snapshot not found")
		return
	}
	if err != nil {
		panic(err)
	}
	ival, err = item.ValueCopy(nil)
	if err != nil {
		panic(err)
	}
	var s common.SnapshotWithTopologicalOrder
	err = common.DecompressMsgpackUnmarshal(ival, &s)
	if err != nil {
		panic(err)
	}
	if s.TopologicalOrder < 4000000 {
		panic(s.TopologicalOrder)
	}
	item, err = txn.Get(graphTopologyKey(s.TopologicalOrder))
	if err != nil {
		panic(err)
	}
	err = txn.Delete(graphTopologyKey(s.TopologicalOrder))
	if err != nil {
		panic(err)
	}
	item, err = txn.Get(graphSnapTopologyKey(snap))
	if err != nil {
		panic(err)
	}
	err = txn.Delete(graphSnapTopologyKey(snap))
	if err != nil {
		panic(err)
	}
	err = txn.Delete(graphSnapshotKey(node, round.Number, tx))
	if err != nil {
		panic(err)
	}

	item, err = txn.Get(graphUniqueKey(node, tx))
	if err != nil {
		panic(err)
	}
	err = txn.Delete(graphUniqueKey(node, tx))
	if err != nil {
		panic(err)
	}

	item, err = txn.Get(append([]byte("FINALIZATION"), tx...))
	if err != nil {
		panic(err)
	}
	ival, err = item.ValueCopy(nil)
	if err != nil {
		panic(err)
	}
	if bytes.Compare(ival, snap) != 0 {
		panic(hex.EncodeToString(ival))
	}
	err = txn.Delete(append([]byte("FINALIZATION"), tx...))
	if err != nil {
		panic(err)
	}

	item, err = txn.Get(graphUtxoKey(tx, 1))
	if err != badger.ErrKeyNotFound {
		panic(err)
	}
	item, err = txn.Get(graphUtxoKey(tx, 0))
	if err != nil {
		panic(err)
	}
	err = txn.Delete(graphUtxoKey(tx, 0))
	if err != nil {
		panic(err)
	}

	item, err = txn.Get(nodePledgeKey(signer))
	if err != badger.ErrKeyNotFound {
		panic(err)
	}
	item, err = txn.Get(nodeAcceptKey(signer))
	if err != nil {
		panic(err)
	}
	err = txn.Delete(nodeAcceptKey(signer))
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(time.Now().Add(-3*24*time.Hour).UnixNano()))
	val := append(payee, pledgeTx...)
	val = append(val, buf...)
	err = txn.Set(nodePledgeKey(signer), val)
	if err != nil {
		panic(err)
	}

	err = txn.Commit()
	if err != nil {
		panic(err)
	}
}

func nodePledgeKey(publicSpend []byte) []byte {
	return append([]byte("NODESTATEPLEDGE"), publicSpend...)
}

func nodeAcceptKey(publicSpend []byte) []byte {
	return append([]byte("NODESTATEACCEPT"), publicSpend...)
}

func graphUtxoKey(hash []byte, index int) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	size := binary.PutVarint(buf, int64(index))
	key := append([]byte("UTXO"), hash...)
	return append(key, buf[:size]...)
}

func graphUniqueKey(nodeId, hash []byte) []byte {
	key := append(hash, nodeId...)
	return append([]byte("UNIQUE"), key...)
}

func graphSnapshotKey(nodeId []byte, round uint64, hash []byte) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, round)
	key := append([]byte("SNAPSHOT"), nodeId...)
	key = append(key, buf...)
	return append(key, hash...)
}

func graphTopologyKey(order uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, order)
	return append([]byte("TOPOLOGY"), buf...)
}

func graphSnapTopologyKey(hash []byte) []byte {
	return append([]byte("SNAPTOPO"), hash...)
}

func openDB(dir string) (*badger.DB, error) {
	opts := badger.DefaultOptions(dir)
	return badger.Open(opts)
}
