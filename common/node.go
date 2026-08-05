package common

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	NodeStatePledging = "PLEDGING"
	NodeStateAccepted = "ACCEPTED"
	NodeStateRemoved  = "REMOVED"
)

type Node struct {
	Signer      Address
	Payee       Address
	State       string
	Transaction crypto.Hash
	Timestamp   uint64
}

func (n *Node) IdForNetwork(networkId crypto.Hash) crypto.Hash {
	return n.Signer.Hash().ForNetwork(networkId)
}

func (tx *Transaction) NodeTransactionExtraAsSigner() *Address {
	switch tx.AsVersioned().TransactionType() {
	case TransactionTypeNodePledge:
	case TransactionTypeNodeAccept:
	case TransactionTypeNodeRemove:
	default:
		panic(tx.AsVersioned().PayloadHash())
	}

	var signer Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	signer.PrivateViewKey = signer.PublicSpendKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	return &signer
}

func (tx *Transaction) validateNodePledge(store DataStore, inputs map[string]*UTXO, snapTime uint64) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for pledge transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 || len(inputs) != len(tx.Inputs) {
		return fmt.Errorf("invalid inputs count %d for pledge transaction", len(tx.Inputs))
	}
	fk := fmt.Sprintf("%s:%d", tx.Inputs[0].Hash.String(), tx.Inputs[0].Index)
	if inputs[fk].Type != OutputTypeScript && inputs[fk].Type != OutputTypeNodeRemove {
		return fmt.Errorf("invalid utxo type %d", inputs[fk].Type)
	}
	if len(tx.Extra) != 2*len(crypto.Key{}) {
		return fmt.Errorf("invalid extra length %d for pledge transaction", len(tx.Extra))
	}

	var signerSpend, payeeSpend crypto.Key
	copy(signerSpend[:], tx.Extra[:32])
	copy(payeeSpend[:], tx.Extra[32:])
	if !signerSpend.CheckKey() || !payeeSpend.CheckKey() {
		return fmt.Errorf("invalid public key %x for pledge transaction", tx.Extra)
	}
	nodes := store.ReadAllNodes(snapTime, false)
	for _, n := range nodes {
		if n.State != NodeStateAccepted && n.State != NodeStateRemoved {
			return fmt.Errorf("invalid node pending state %s %s", n.Signer.String(), n.State)
		}
		if n.Signer.PublicSpendKey.String() == signerSpend.String() {
			return fmt.Errorf("invalid node signer key %s %s", hex.EncodeToString(tx.Extra), n.Signer)
		}
		if n.Payee.PublicSpendKey.String() == signerSpend.String() {
			return fmt.Errorf("invalid node signer key %s %s", hex.EncodeToString(tx.Extra), n.Payee)
		}
	}

	return nil
}

func (tx *Transaction) validateNodeAccept(store DataStore, payloadHash crypto.Hash, sigs []map[uint16]*crypto.Signature, snapTime uint64) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for accept transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for accept transaction", len(tx.Inputs))
	}
	if len(sigs) != 1 || len(sigs[0]) != 1 || sigs[0][0] == nil {
		return fmt.Errorf("invalid signatures %v for accept transaction", sigs)
	}
	var pledging *Node
	filter := make(map[string]string)
	nodes := store.ReadAllNodes(snapTime, false)
	for _, n := range nodes {
		filter[n.Signer.String()] = n.State
		if n.State == NodeStateAccepted || n.State == NodeStateRemoved {
			continue
		}
		if n.State == NodeStatePledging && pledging == nil {
			pledging = n
		} else {
			return fmt.Errorf("invalid pledging nodes %s %s", pledging.Signer.String(), n.Signer.String())
		}
	}
	if pledging == nil {
		return fmt.Errorf("no pledging node needs to get accepted")
	}
	if pledging.Transaction != tx.Inputs[0].Hash {
		return fmt.Errorf("invalid pledge utxo source %s %s", pledging.Transaction, tx.Inputs[0].Hash)
	}

	lastPledge, _, err := store.ReadTransaction(tx.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if len(lastPledge.Outputs) != 1 {
		return fmt.Errorf("invalid pledge utxo count %d", len(lastPledge.Outputs))
	}
	po := lastPledge.Outputs[0]
	if po.Type != OutputTypeNodePledge {
		return fmt.Errorf("invalid pledge utxo type %d", po.Type)
	}
	acc := lastPledge.NodeTransactionExtraAsSigner()
	if filter[acc.String()] != NodeStatePledging {
		return fmt.Errorf("invalid pledge utxo source %s", filter[acc.String()])
	}
	if !bytes.Equal(lastPledge.Extra, tx.Extra) {
		return fmt.Errorf("invalid pledge and accept key %s %s", hex.EncodeToString(lastPledge.Extra), hex.EncodeToString(tx.Extra))
	}
	if !acc.PublicSpendKey.Verify(payloadHash, *sigs[0][0]) {
		return fmt.Errorf("invalid accept signature %s", sigs[0][0])
	}
	return nil
}

func (tx *Transaction) validateNodeRemove(store DataStore) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for remove transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for remove transaction", len(tx.Inputs))
	}

	accept, _, err := store.ReadTransaction(tx.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if accept.PayloadHash() != tx.Inputs[0].Hash {
		return fmt.Errorf("accept transaction malformed %s %s", tx.Inputs[0].Hash, accept.PayloadHash())
	}
	if len(accept.Outputs) != 1 {
		return fmt.Errorf("invalid accept utxo count %d", len(accept.Outputs))
	}
	ao := accept.Outputs[0]
	if ao.Type != OutputTypeNodeAccept {
		return fmt.Errorf("invalid accept utxo type %d", ao.Type)
	}
	if !bytes.Equal(accept.Extra, tx.Extra) {
		return fmt.Errorf("invalid accept and remove key %s %s", hex.EncodeToString(accept.Extra), hex.EncodeToString(tx.Extra))
	}
	return nil
}
