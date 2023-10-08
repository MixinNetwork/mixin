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
	custodianNodeActionUpdate  = 1
	custodianNodesMinimumCount = 7
	custodianNodeNewPrice      = 100
	custodianNodeUpdatePrice   = 1
)

type CustodianUpdateRequest struct {
	Custodian   *Address
	Nodes       []*CustodianNode
	Signature   *crypto.Signature
	Transaction crypto.Hash
	Timestamp   uint64
}

type CustodianNode struct {
	Custodian Address
	Payee     Address
	Extra     []byte
}

func (cn *CustodianNode) validate() error {
	if cn.Payee.PublicSpendKey == cn.Custodian.PublicSpendKey {
		return fmt.Errorf("invalid custodian or payee keys %x", cn.Extra)
	}

	eh := crypto.Blake3Hash(cn.Extra[:161])
	var payeeSig, custodianSig crypto.Signature
	copy(payeeSig[:], cn.Extra[225:289])
	copy(custodianSig[:], cn.Extra[289:custodianNodeExtraSize])
	if !cn.Payee.PublicSpendKey.Verify(eh, payeeSig) {
		return fmt.Errorf("invalid custodian update payee signature %x", cn.Extra)
	}
	if !cn.Custodian.PublicSpendKey.Verify(eh, custodianSig) {
		return fmt.Errorf("invalid custodian update custodian signature %x", cn.Extra)
	}
	return nil
}

func EncodeCustodianNode(custodian, payee *Address, signerSpend, payeeSpend, custodianSpend *crypto.Key, networkId crypto.Hash) []byte {
	signer := Address{
		PublicSpendKey: signerSpend.Public(),
		PublicViewKey:  signerSpend.Public().DeterministicHashDerive().Public(),
	}
	nodeId := signer.Hash().ForNetwork(networkId)

	extra := []byte{custodianNodeActionUpdate}
	extra = append(extra, custodian.PublicSpendKey[:]...)
	extra = append(extra, custodian.PublicViewKey[:]...)
	extra = append(extra, payee.PublicSpendKey[:]...)
	extra = append(extra, payee.PublicViewKey[:]...)
	extra = append(extra, nodeId[:]...)

	eh := crypto.Blake3Hash(extra)
	signerSig := signerSpend.Sign(eh)
	payeeSig := payeeSpend.Sign(eh)
	custodianSig := custodianSpend.Sign(eh)
	extra = append(extra, signerSig[:]...)
	extra = append(extra, payeeSig[:]...)
	extra = append(extra, custodianSig[:]...)
	return extra
}

func ParseCustodianNode(extra []byte) (*CustodianNode, error) {
	if len(extra) != custodianNodeExtraSize {
		return nil, fmt.Errorf("invalid custodian node data %x", extra)
	}
	if extra[0] != custodianNodeActionUpdate {
		return nil, fmt.Errorf("invalid custodian update action %x", extra)
	}
	var cn CustodianNode
	cn.Extra = make([]byte, len(extra))
	copy(cn.Extra, extra)
	copy(cn.Custodian.PublicSpendKey[:], extra[1:33])
	copy(cn.Custodian.PublicViewKey[:], extra[33:65])
	copy(cn.Payee.PublicSpendKey[:], extra[65:97])
	copy(cn.Payee.PublicViewKey[:], extra[97:129])
	err := cn.validate()
	if err != nil {
		return nil, err
	}
	return &cn, nil
}

func ParseCustodianUpdateNodesExtra(extra []byte) (*CustodianUpdateRequest, error) {
	if len(extra) < 64+custodianNodeExtraSize*custodianNodesMinimumCount+64 {
		return nil, fmt.Errorf("invalid custodian update extra %x", extra)
	}
	var custodian Address
	copy(custodian.PublicSpendKey[:], extra[:32])
	copy(custodian.PublicViewKey[:], extra[32:64])
	var prevCustodianSig crypto.Signature
	copy(prevCustodianSig[:], extra[len(extra)-64:])

	// 1 || custodian (Address) || payee (Address) || node id (Hash) || signerSig || payeeSig || custodianSig
	nodesExtra := extra[64 : len(extra)-64]
	if len(nodesExtra)%custodianNodeExtraSize != 0 {
		return nil, fmt.Errorf("invalid custodian update extra %x", extra)
	}
	nodes := make([]*CustodianNode, len(nodesExtra)/custodianNodeExtraSize)
	uniqueKeys := make(map[crypto.Key]bool)
	for i := range nodes {
		cne := nodesExtra[i*custodianNodeExtraSize : (i+1)*custodianNodeExtraSize]
		cn, err := ParseCustodianNode(cne)
		if err != nil {
			return nil, err
		}
		if uniqueKeys[cn.Payee.PublicSpendKey] || uniqueKeys[cn.Custodian.PublicSpendKey] {
			return nil, fmt.Errorf("duplicate custodian or payee keys %x", cne)
		}
		uniqueKeys[cn.Payee.PublicSpendKey] = true
		uniqueKeys[cn.Payee.PublicViewKey] = true
		uniqueKeys[cn.Custodian.PublicSpendKey] = true
		uniqueKeys[cn.Custodian.PublicViewKey] = true
		nodes[i] = cn
	}

	var sortedExtra []byte
	sort.Slice(nodes, func(i, j int) bool {
		return bytes.Compare(nodes[i].Custodian.PublicSpendKey[:], nodes[j].Custodian.PublicSpendKey[:]) < 0
	})
	for _, n := range nodes {
		sortedExtra = append(sortedExtra, n.Extra...)
	}
	if !bytes.Equal(nodesExtra, sortedExtra) {
		return nil, fmt.Errorf("invalid custodian nodes extra sort order %x", extra)
	}

	return &CustodianUpdateRequest{
		Custodian: &custodian,
		Nodes:     nodes,
		Signature: &prevCustodianSig,
	}, nil
}

func (tx *Transaction) validateCustodianUpdateNodes(store CustodianReader) error {
	if tx.Version < TxVersionHashSignature {
		return fmt.Errorf("invalid custodian update version %d", tx.Version)
	}
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

	curs, err := ParseCustodianUpdateNodesExtra(tx.Extra)
	if err != nil {
		return err
	}
	if len(curs.Nodes) < custodianNodesMinimumCount {
		return fmt.Errorf("invalid custodian nodes count %d", len(curs.Nodes))
	}

	now := uint64(time.Now().UnixNano())
	prev, err := store.ReadCustodian(now)
	if err != nil {
		return err
	}
	if prev == nil {
		// FIXME do genesis check
		// in the genesis, will assign a custodian directly, and all nodes are required
		// to be in the safe custodian network
		panic("FIXME genesis check")
	}
	eh := crypto.Blake3Hash(tx.Extra[:len(tx.Extra)-64])
	if !prev.Custodian.PublicSpendKey.Verify(eh, *curs.Signature) {
		return fmt.Errorf("invalid custodian update approval signature %x", tx.Extra)
	}

	filter := make(map[string]string)
	for _, n := range prev.Nodes {
		filter[n.Custodian.String()] = n.Payee.String()
	}
	if len(filter) != len(prev.Nodes) {
		panic(prev.Custodian.String())
	}
	total := Zero
	newPrice := NewInteger(custodianNodeNewPrice)
	udpatePrice := NewInteger(custodianNodeUpdatePrice)
	for _, n := range curs.Nodes {
		old, found := filter[n.Custodian.String()]
		if !found {
			total = total.Add(newPrice)
		} else if old != n.Payee.String() {
			total = total.Add(udpatePrice)
		}
		delete(filter, n.Custodian.String())
	}
	if out.Amount.Cmp(total) < 0 {
		return fmt.Errorf("invalid custodian nodes update price %v", out)
	}

	if curs.Custodian.String() != prev.Custodian.String() {
		return nil
	}
	if len(filter) != 0 || len(prev.Nodes) != len(curs.Nodes) {
		return fmt.Errorf("custodian account and nodes mismatch %x", tx.Extra)
	}
	return nil
}

func (tx *Transaction) validateCustodianSlashNodes(store DataStore) error {
	return fmt.Errorf("not implemented %v", tx)
}
