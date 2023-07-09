package common

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	custodianNodeExtraSize     = 353
	custodianNodesUpdateAction = 1
	custodianNodesMinimumCount = 7
	custodianNodePrice         = 100
)

type CustodianNode struct {
	Custodian Address
	Payee     Address
	Extra     []byte
}

func (cn *CustodianNode) validate() error {
	if cn.Payee.PublicSpendKey == cn.Custodian.PublicSpendKey {
		return fmt.Errorf("invalid custodian or payee keys %x", cn.Extra)
	}

	var payeeSig, custodianSig crypto.Signature
	copy(payeeSig[:], cn.Extra[225:289])
	copy(custodianSig[:], cn.Extra[289:custodianNodeExtraSize])
	if !cn.Payee.PublicSpendKey.Verify(cn.Extra[:161], payeeSig) {
		return fmt.Errorf("invalid custodian update payee signature %x", cn.Extra)
	}
	if !cn.Custodian.PublicSpendKey.Verify(cn.Extra[:161], custodianSig) {
		return fmt.Errorf("invalid custodian update custodian signature %x", cn.Extra)
	}
	return nil
}

func (tx *Transaction) parseCustodianUpdateNodesExtra() (*Address, []*CustodianNode, *crypto.Signature, error) {
	if len(tx.Extra) < 64+custodianNodeExtraSize*custodianNodesMinimumCount+64 {
		return nil, nil, nil, fmt.Errorf("invalid custodian update extra %x", tx.Extra)
	}
	var custodian Address
	copy(custodian.PublicSpendKey[:], tx.Extra[:32])
	copy(custodian.PublicViewKey[:], tx.Extra[32:64])
	var prevCustodianSig crypto.Signature
	copy(prevCustodianSig[:], tx.Extra[len(tx.Extra)-64:])

	// 1 || custodian (Address) || payee (Address) || node id (Hash) || signerSig || payeeSig || custodianSig
	nodesExtra := tx.Extra[64 : len(tx.Extra)-64]
	if len(nodesExtra)%custodianNodeExtraSize != 0 {
		return nil, nil, nil, fmt.Errorf("invalid custodian update extra %x", tx.Extra)
	}
	nodes := make([]*CustodianNode, len(nodesExtra)/custodianNodeExtraSize)
	uniqueKeys := make(map[crypto.Key]bool)
	for i := range nodes {
		extra := nodesExtra[i*custodianNodeExtraSize : (i+1)*custodianNodeExtraSize]
		if extra[0] != custodianNodesUpdateAction {
			return nil, nil, nil, fmt.Errorf("invalid custodian update action %x", tx.Extra)
		}
		var cn CustodianNode
		copy(cn.Custodian.PublicSpendKey[:], extra[1:33])
		copy(cn.Custodian.PublicViewKey[:], extra[33:65])
		copy(cn.Payee.PublicSpendKey[:], extra[65:97])
		copy(cn.Payee.PublicViewKey[:], extra[97:129])
		copy(cn.Extra, extra[:custodianNodeExtraSize])
		if uniqueKeys[cn.Payee.PublicSpendKey] || uniqueKeys[cn.Custodian.PublicSpendKey] {
			return nil, nil, nil, fmt.Errorf("duplicate custodian or payee keys %x", tx.Extra)
		}
		uniqueKeys[cn.Payee.PublicSpendKey] = true
		uniqueKeys[cn.Payee.PublicViewKey] = true
		uniqueKeys[cn.Custodian.PublicSpendKey] = true
		uniqueKeys[cn.Custodian.PublicViewKey] = true
		err := cn.validate()
		if err != nil {
			return nil, nil, nil, err
		}
		nodes[i] = &cn
	}

	var sortedExtra []byte
	sort.Slice(nodes, func(i, j int) bool {
		return bytes.Compare(nodes[i].Custodian.PublicSpendKey[:], nodes[j].Custodian.PublicSpendKey[:]) < 0
	})
	for _, n := range nodes {
		sortedExtra = append(sortedExtra, n.Extra...)
	}
	if !bytes.Equal(nodesExtra, sortedExtra) {
		return nil, nil, nil, fmt.Errorf("invalid custodian nodes extra sort order %x", tx.Extra)
	}

	return &custodian, nodes, &prevCustodianSig, nil
}

func (tx *Transaction) validateCustodianUpdateNodes(store DataStore) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid custodian update asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid custodian update outputs count %d", len(tx.Outputs))
	}
	out := tx.Outputs[0]
	if out.Type != OutputTypeCustodianUpdateNodes {
		return fmt.Errorf("invalid custodian update output type %v", out)
	}
	if len(out.Keys) != 1 || out.Script.String() != "fffe40" {
		return fmt.Errorf("invalid custodian update output receiver %v", out)
	}

	custodian, custodianNodes, prevCustodianSig, err := tx.parseCustodianUpdateNodesExtra()
	if err != nil {
		return err
	}
	if len(custodianNodes) < custodianNodesMinimumCount {
		return fmt.Errorf("invalid custodian nodes count %d", len(custodianNodes))
	}
	if out.Amount.Cmp(NewInteger(custodianNodePrice).Mul(len(custodianNodes))) != 0 {
		return fmt.Errorf("invalid custodian nodes update price %v", out)
	}

	now := uint64(time.Now().UnixNano())
	prevCustodian, prevNodes, _, err := store.ReadCustodian(now)
	if err != nil {
		return err
	}
	if prevCustodian == nil {
		domains := store.ReadDomains()
		if len(domains) != 1 {
			return fmt.Errorf("invalid domains count %d", len(domains))
		}
		prevCustodian = &domains[0].Account
	}
	if !prevCustodian.PublicSpendKey.Verify(tx.Extra[:len(tx.Extra)-64], *prevCustodianSig) {
		return fmt.Errorf("invalid custodian update approval signature %x", tx.Extra)
	}

	if custodian.String() != prevCustodian.String() {
		return nil
	}
	var prevExtra []byte
	for _, n := range prevNodes {
		prevExtra = append(prevExtra, n.Extra...)
	}
	if !bytes.Equal(prevExtra, tx.Extra[64:len(tx.Extra)-64]) {
		return fmt.Errorf("custodian account and nodes mismatch %x", tx.Extra)
	}
	return nil
}

func (tx *Transaction) validateCustodianSlashNodes(store DataStore) error {
	return fmt.Errorf("not implemented %v", tx)
}
