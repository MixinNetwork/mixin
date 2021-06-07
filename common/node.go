package common

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
)

const (
	NodeStatePledging  = "PLEDGING"
	NodeStateAccepted  = "ACCEPTED"
	NodeStateRemoved   = "REMOVED"
	NodeStateCancelled = "CANCELLED"
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

func (tx *Transaction) validateNodePledge(store DataStore, inputs map[string]*UTXO) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for pledge transaction", len(tx.Outputs))
	}
	if len(tx.Extra) != 2*len(crypto.Key{}) {
		return fmt.Errorf("invalid extra length %d for pledge transaction", len(tx.Extra))
	}
	for _, in := range inputs {
		if in.Type != OutputTypeScript {
			return fmt.Errorf("invalid utxo type %d", in.Type)
		}
	}

	var signerSpend crypto.Key
	copy(signerSpend[:], tx.Extra)
	nodes := store.ReadAllNodes(uint64(time.Now().UnixNano()), false) // FIXME offset incorrect
	for _, n := range nodes {
		if n.State != NodeStateAccepted && n.State != NodeStateCancelled && n.State != NodeStateRemoved {
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

func (tx *Transaction) validateNodeCancel(store DataStore, msg []byte, sigs []map[uint16]*crypto.Signature) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 2 {
		return fmt.Errorf("invalid outputs count %d for cancel transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for cancel transaction", len(tx.Inputs))
	}
	if len(sigs) != 1 || len(sigs[0]) != 1 || sigs[0][0] == nil {
		return fmt.Errorf("invalid signatures %v for cancel transaction", sigs)
	}
	if len(tx.Extra) != len(crypto.Key{})*3 {
		return fmt.Errorf("invalid extra %s for cancel transaction", hex.EncodeToString(tx.Extra))
	}
	cancel, script := tx.Outputs[0], tx.Outputs[1]
	if cancel.Type != OutputTypeNodeCancel || script.Type != OutputTypeScript {
		return fmt.Errorf("invalid outputs type %d %d for cancel transaction", cancel.Type, script.Type)
	}
	if len(script.Keys) != 1 {
		return fmt.Errorf("invalid script output keys %d for cancel transaction", len(script.Keys))
	}
	if script.Script.String() != NewThresholdScript(1).String() {
		return fmt.Errorf("invalid script output script %s for cancel transaction", script.Script)
	}

	var pledging *Node
	filter := make(map[string]string)
	nodes := store.ReadAllNodes(uint64(time.Now().UnixNano()), false) // FIXME offset incorrect
	for _, n := range nodes {
		filter[n.Signer.String()] = n.State
		if n.State == NodeStateAccepted || n.State == NodeStateCancelled || n.State == NodeStateRemoved {
			continue
		}
		if n.State == NodeStatePledging && pledging == nil {
			pledging = n
		} else {
			return fmt.Errorf("invalid pledging nodes %s %s", pledging.Signer.String(), n.Signer.String())
		}
	}
	if pledging == nil {
		return fmt.Errorf("no pledging node needs to get cancelled")
	}
	if pledging.Transaction != tx.Inputs[0].Hash {
		return fmt.Errorf("invalid plede utxo source %s %s", pledging.Transaction, tx.Inputs[0].Hash)
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
	if cancel.Amount.Cmp(po.Amount.Div(100)) != 0 {
		return fmt.Errorf("invalid script output amount %s for cancel transaction", cancel.Amount)
	}
	var publicSpend crypto.Key
	copy(publicSpend[:], lastPledge.Extra)
	privateView := publicSpend.DeterministicHashDerive()
	acc := Address{
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
	if filter[acc.String()] != NodeStatePledging {
		return fmt.Errorf("invalid pledge utxo source %s", filter[acc.String()])
	}

	pit, _, err := store.ReadTransaction(lastPledge.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if pit == nil {
		return fmt.Errorf("invalid pledge input source %s:%d", lastPledge.Inputs[0].Hash, lastPledge.Inputs[0].Index)
	}
	pi := pit.Outputs[lastPledge.Inputs[0].Index]
	if len(pi.Keys) != 1 {
		return fmt.Errorf("invalid pledge input source keys %d", len(pi.Keys))
	}
	var a crypto.Key
	copy(a[:], tx.Extra[len(crypto.Key{})*2:])
	pledgeSpend := crypto.ViewGhostOutputKey(pi.Keys[0], &a, &pi.Mask, uint64(lastPledge.Inputs[0].Index))
	targetSpend := crypto.ViewGhostOutputKey(script.Keys[0], &a, &script.Mask, 1)
	if !bytes.Equal(lastPledge.Extra, tx.Extra[:len(crypto.Key{})*2]) {
		return fmt.Errorf("invalid pledge and cancel key %s %s", hex.EncodeToString(lastPledge.Extra), hex.EncodeToString(tx.Extra))
	}
	if !bytes.Equal(pledgeSpend[:], targetSpend[:]) {
		return fmt.Errorf("invalid pledge and cancel target %s %s", pledgeSpend, targetSpend)
	}
	if !pi.Keys[0].Verify(msg, *sigs[0][0]) {
		return fmt.Errorf("invalid cancel signature %s", sigs[0][0])
	}
	return nil
}

func (tx *Transaction) validateNodeAccept(store DataStore) error {
	if tx.Asset != XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for accept transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for accept transaction", len(tx.Inputs))
	}
	var pledging *Node
	filter := make(map[string]string)
	nodes := store.ReadAllNodes(uint64(time.Now().UnixNano()), false) // FIXME offset incorrect
	for _, n := range nodes {
		filter[n.Signer.String()] = n.State
		if n.State == NodeStateAccepted || n.State == NodeStateCancelled || n.State == NodeStateRemoved {
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
		return fmt.Errorf("invalid plede utxo source %s %s", pledging.Transaction, tx.Inputs[0].Hash)
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
	var publicSpend crypto.Key
	copy(publicSpend[:], lastPledge.Extra)
	privateView := publicSpend.DeterministicHashDerive()
	acc := Address{
		PublicViewKey:  privateView.Public(),
		PublicSpendKey: publicSpend,
	}
	if filter[acc.String()] != NodeStatePledging {
		return fmt.Errorf("invalid pledge utxo source %s", filter[acc.String()])
	}
	if !bytes.Equal(lastPledge.Extra, tx.Extra) {
		return fmt.Errorf("invalid pledge and accpet key %s %s", hex.EncodeToString(lastPledge.Extra), hex.EncodeToString(tx.Extra))
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
