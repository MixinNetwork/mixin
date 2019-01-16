package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
)

const (
	snapshotsPrefixDomainAccept = "DOMAINACCEPT"
	snapshotsPrefixDomainRemove = "DOMAINREMOVE"
)

func (s *BadgerStore) SnapshotsReadDomains() []common.Domain {
	domains := make([]common.Domain, 0)
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	prefix := []byte(snapshotsPrefixDomainAccept)
	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		acc := domainAccountForState(it.Item().Key(), snapshotsPrefixDomainAccept)
		domains = append(domains, common.Domain{Account: acc})
	}
	return domains
}

func writeDomainAccept(txn *badger.Txn, publicSpend crypto.Key, tx crypto.Hash) error {
	key := domainAcceptKey(publicSpend)
	return txn.Set(key, tx[:])
}

func domainAccountForState(key []byte, domainState string) common.Address {
	var publicSpend crypto.Key
	copy(publicSpend[:], key[len(domainState):])
	privateView := publicSpend.DeterministicHashDerive()
	return common.Address{
		PrivateViewKey: privateView,
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
}

func domainAcceptKey(publicSpend crypto.Key) []byte {
	return append([]byte(snapshotsPrefixDomainAccept), publicSpend[:]...)
}
