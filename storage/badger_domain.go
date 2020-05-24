package storage

import (
	"encoding/binary"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger/v2"
)

const (
	graphPrefixDomainAccept = "DOMAINACCEPT"
	graphPrefixDomainRemove = "DOMAINREMOVE"
)

func (s *BadgerStore) ReadDomains() []common.Domain {
	domains := make([]common.Domain, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(graphPrefixDomainAccept)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		acc := domainAccountForState(it.Item().Key(), graphPrefixDomainAccept)
		domains = append(domains, common.Domain{Account: acc})
	}
	return domains
}

func writeDomainAccept(txn *badger.Txn, publicSpend crypto.Key, tx crypto.Hash, timestamp uint64) error {
	key := graphDomainAcceptKey(publicSpend)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, timestamp)
	val := append(tx[:], buf...)
	return txn.Set(key, val)
}

func domainAccountForState(key []byte, domainState string) common.Address {
	var publicSpendKey crypto.Key
	copy(publicSpendKey[:], key[len(domainState):])
	publicSpend := publicSpendKey.AsPublicKeyOrPanic()
	privateView := publicSpend.DeterministicHashDerive()
	return common.Address{
		PrivateViewKey: privateView,
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
}

func graphDomainAcceptKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixDomainAccept), publicSpend[:]...)
}
