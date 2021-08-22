package main

import (
	"flag"
	"os"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v3"
)

func main() {
	dbDir := flag.String("db", "/tmp/mixin/snapshots", "the mixin badger snapshots directory")
	dicDir := flag.String("dic", "/tmp/zstd", "the directory to store zstd dictionary samples")
	flag.Parse()

	db, err := openDB(*dbDir)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	loopUTXOs(db, *dicDir)
	loopTransactions(db, *dicDir)
	loopSnapshots(db, *dicDir)
}

func loopSnapshots(db *badger.DB, dir string) {
	txn := db.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Seek([]byte("SNAPSHOT"))
	for ; it.ValidForPrefix([]byte("SNAPSHOT")); it.Next() {
		item := it.Item()
		key := item.Key()
		val, err := item.ValueCopy(nil)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(dir+"/SNAPSHOT-"+crypto.NewHash(key).String(), val, 0644)
		if err != nil {
			panic(err)
		}
	}
}

func loopTransactions(db *badger.DB, dir string) {
	txn := db.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Seek([]byte("TRANSACTION"))
	for ; it.ValidForPrefix([]byte("TRANSACTION")); it.Next() {
		item := it.Item()
		key := item.Key()
		val, err := item.ValueCopy(nil)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(dir+"/TRANSACTION-"+crypto.NewHash(key).String(), val, 0644)
		if err != nil {
			panic(err)
		}
	}
}

func loopUTXOs(db *badger.DB, dir string) {
	txn := db.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	it.Seek([]byte("UTXO"))
	for ; it.ValidForPrefix([]byte("UTXO")); it.Next() {
		item := it.Item()
		key := item.Key()
		val, err := item.ValueCopy(nil)
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(dir+"/UTXO-"+crypto.NewHash(key).String(), val, 0644)
		if err != nil {
			panic(err)
		}
	}
}

func openDB(dir string) (*badger.DB, error) {
	opts := badger.DefaultOptions(dir)
	return badger.Open(opts)
}
