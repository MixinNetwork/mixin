package storage

import (
	"github.com/MixinNetwork/mixin/common"
	"github.com/dgraph-io/badger/v4"
)

func (s *BadgerStore) writeCustodianNodes(txn *badger.Txn, snap *common.Snapshot, custodian *common.Address, nodes []*common.CustodianInfo) error {
	panic(0)
}

func (s *BadgerStore) ReadCustodianAccount() (*common.Address, error) {
	txn := s.snapshotsDB.NewTransaction(false)
	defer txn.Discard()

	item, err := txn.Get([]byte(graphPrefixCustodianAccount))
	if err == badger.ErrKeyNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	val, err := item.ValueCopy(nil)
	if err != nil {
		return nil, err
	}
	addr, err := common.NewAddressFromString(string(val))
	if err != nil {
		return nil, err
	}
	return &addr, nil
}
