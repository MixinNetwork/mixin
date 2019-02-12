package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/dgraph-io/badger"
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

func writeDomainAccept(txn *badger.Txn, publicSpend crypto.Key, tx crypto.Hash) error {
	key := graphDomainAcceptKey(publicSpend)
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

func graphDomainAcceptKey(publicSpend crypto.Key) []byte {
	return append([]byte(graphPrefixDomainAccept), publicSpend[:]...)
}
