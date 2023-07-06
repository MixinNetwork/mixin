package common

import "fmt"

type CustodianInfo struct {
	Custodian Address
	Payee     Address
	Extra     []byte
}

func (ci *CustodianInfo) Validate() error {
	panic(0)
}

func (tx *Transaction) validateCustodianUpdateNodes(store DataStore) error {
	return fmt.Errorf("not implemented %v", tx)
}

func (tx *Transaction) validateCustodianSlashNodes(store DataStore) error {
	return fmt.Errorf("not implemented %v", tx)
}
