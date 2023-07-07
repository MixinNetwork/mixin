package common

import "fmt"

type CustodianNode struct {
	Custodian Address
	Payee     Address
	Extra     []byte
}

func (ci *CustodianNode) validate() error {
	panic(0)
}

func (tx *Transaction) parseCustodianUpdateNodesExtra() (*Address, []*CustodianNode, error) {
	panic(0)
}

func (tx *Transaction) validateCustodianUpdateNodes(store DataStore) error {
	return fmt.Errorf("not implemented %v", tx)
}

func (tx *Transaction) validateCustodianSlashNodes(store DataStore) error {
	return fmt.Errorf("not implemented %v", tx)
}
