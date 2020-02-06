package kernel

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/logger"
)

func (node *Node) ElectionLoop() error {
	for node.Graph.MyCacheRound == nil {
		time.Sleep(13 * time.Minute)
		now := uint64(time.Now().UnixNano())
		if now < node.epoch {
			logger.Printf("LOCAL TIME INVALID %d %d\n", now, node.epoch)
			continue
		}
		hours := int((now-node.epoch)/3600000000000) % 24
		if hours < config.KernelNodeAcceptTimeBegin || hours > config.KernelNodeAcceptTimeEnd {
			continue
		}

		err := node.tryToSendAcceptTransaction()
		if err != nil {
			logger.Println("tryToSendAcceptTransaction", err)
		}
	}
	logger.Println("ElectionLoop DONE")
	return nil
}

func (node *Node) tryToSendAcceptTransaction() error {
	pledging := node.ConsensusPledging
	if pledging == nil {
		return fmt.Errorf("no consensus pledging node")
	}
	if pledging.Signer.String() != node.Signer.String() {
		return fmt.Errorf("invalid consensus pledging node %s %s", pledging.Signer, node.Signer)
	}
	pledge, _, err := node.persistStore.ReadTransaction(pledging.Transaction)
	if err != nil {
		return err
	}
	if pledge == nil {
		return fmt.Errorf("pledge transaction not available yet %s", pledging.Transaction)
	}
	if pledge.PayloadHash() != pledging.Transaction {
		return fmt.Errorf("pledge transaction malformed %s %s", pledging.Transaction, pledge.PayloadHash())
	}
	signer := node.Signer.PublicSpendKey
	if len(pledge.Extra) != len(signer)*2 {
		return fmt.Errorf("invalid pledge transaction extra %s", hex.EncodeToString(pledge.Extra))
	}
	if bytes.Compare(signer[:], pledge.Extra[:len(signer)]) != 0 {
		return fmt.Errorf("invalid pledge transaction extra %s %s", hex.EncodeToString(pledge.Extra[:len(signer)]), signer)
	}

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddInput(pledging.Transaction, 0)
	tx.AddOutputWithType(common.OutputTypeNodeAccept, nil, common.Script{}, pledge.Outputs[0].Amount, []byte{})
	tx.Extra = pledge.Extra
	ver := tx.AsLatestVersion()

	err = ver.Validate(node.persistStore)
	if err != nil {
		return err
	}
	err = node.persistStore.CachePutTransaction(ver)
	if err != nil {
		return err
	}
	err = node.persistStore.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: ver.PayloadHash(),
	}, false)
	logger.Println("tryToSendAcceptTransaction", ver.PayloadHash(), hex.EncodeToString(ver.Marshal()))
	return nil
}

func (node *Node) reloadConsensusNodesList(s *common.Snapshot, tx *common.VersionedTransaction) error {
	switch tx.TransactionType() {
	case common.TransactionTypeNodePledge,
		common.TransactionTypeNodeCancel,
		common.TransactionTypeNodeAccept,
		common.TransactionTypeNodeResign,
		common.TransactionTypeNodeRemove:
		err := node.LoadConsensusNodes()
		if err != nil {
			return err
		}
		graph, err := LoadRoundGraph(node.persistStore, node.networkId, node.IdForNetwork)
		if err != nil {
			return err
		}
		node.Graph = graph
	}
	return nil
}

func (node *Node) finalizeNodeAcceptSnapshot(s *common.Snapshot) error {
	cache := &CacheRound{
		NodeId:    s.NodeId,
		Number:    s.RoundNumber,
		Timestamp: s.Timestamp,
	}
	if !cache.ValidateSnapshot(s, true) {
		panic("should never be here")
	}
	err := node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, cache.Timestamp)
	if err != nil {
		panic(err)
	}
	topo := &common.SnapshotWithTopologicalOrder{
		Snapshot:         *s,
		TopologicalOrder: node.TopoCounter.Next(),
	}
	err = node.persistStore.WriteSnapshot(topo)
	if err != nil {
		panic(err)
	}

	final := cache.asFinal()
	external, err := node.getInitialExternalReference(s)
	if err != nil {
		panic(err)
	}
	cache = &CacheRound{
		NodeId:    s.NodeId,
		Number:    1,
		Timestamp: s.Timestamp + config.SnapshotRoundGap + 1,
		References: &common.RoundLink{
			Self:     final.Hash,
			External: external.Hash,
		},
	}
	err = node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, cache.Timestamp)
	if err != nil {
		panic(err)
	}

	node.assignNewGraphRound(final, cache)
	return nil
}

func (node *Node) getInitialExternalReference(s *common.Snapshot) (*FinalRound, error) {
	nodeDistance := func(a, b crypto.Hash) int {
		ai := new(big.Int).SetBytes(a[:])
		bi := new(big.Int).SetBytes(b[:])
		si := new(big.Int).Sub(ai, bi)
		ai = new(big.Int).Abs(si)
		mi := new(big.Int).Mod(ai, big.NewInt(100))
		return int(mi.Int64())
	}

	externalId := node.genesisNodes[0]
	distance := nodeDistance(s.NodeId, externalId)
	for _, id := range node.genesisNodes {
		nd := nodeDistance(s.NodeId, id)
		if nd < distance {
			distance = nd
			externalId = id
		}
	}

	return loadFinalRoundForNode(node.persistStore, externalId, 0)
}

func (node *Node) validateNodePledgeSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	timestamp := s.Timestamp
	if s.Timestamp == 0 && s.NodeId == node.IdForNetwork {
		timestamp = uint64(time.Now().UnixNano())
	}
	for _, cn := range node.ConsensusNodes {
		if timestamp < cn.Timestamp {
			return fmt.Errorf("invalid snapshot timestamp %d %d", cn.Timestamp, timestamp)
		}
		elapse := time.Duration(timestamp - cn.Timestamp)
		if elapse < config.KernelNodePledgePeriodMinimum {
			return fmt.Errorf("invalid pledge period %d %d", config.KernelNodePledgePeriodMinimum, elapse)
		}
		if cn.State != common.NodeStateAccepted {
			return fmt.Errorf("invalid node state %s %s", cn.Signer, cn.State)
		}
	}

	if timestamp < node.epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.epoch, timestamp)
	}
	since := timestamp - node.epoch
	days := int(since / 3600000000000 / 24)
	elp := time.Duration((days%MintYearBatches)*24) * time.Hour
	eta := time.Duration((MintYearBatches-days%MintYearBatches)*24) * time.Hour
	if eta < config.KernelNodeAcceptPeriodMaximum*2 || elp < config.KernelNodeAcceptPeriodMinimum*2 {
		return fmt.Errorf("invalid pledge timestamp %d %d", eta, elp)
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if timestamp > uint64(time.Now().UnixNano())+threshold {
		return fmt.Errorf("invalid snapshot timestamp %d %d", time.Now().UnixNano(), timestamp)
	}
	if cn := node.ConsensusPledging; cn != nil {
		return fmt.Errorf("invalid node state %s %s", cn.Signer, cn.State)
	}
	if tx.Asset != common.XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for pledge transaction", len(tx.Outputs))
	}
	if len(tx.Extra) != 2*len(crypto.Key{}) {
		return fmt.Errorf("invalid extra length %d for pledge transaction", len(tx.Extra))
	}
	if tx.Outputs[0].Amount.Cmp(pledgeAmount(time.Duration(since))) != 0 {
		return fmt.Errorf("invalid pledge amount %s", tx.Outputs[0].Amount.String())
	}

	var signerSpend, payeeSpend crypto.Key
	copy(signerSpend[:], tx.Extra)
	copy(payeeSpend[:], tx.Extra[len(signerSpend):])
	for _, n := range node.persistStore.ReadAllNodes() {
		if n.State != common.NodeStateAccepted && n.State != common.NodeStateCancelled && n.State != common.NodeStateRemoved {
			return fmt.Errorf("invalid node pending state %s %s", n.Signer.String(), n.State)
		}
		if n.Signer.PublicSpendKey.String() == signerSpend.String() {
			return fmt.Errorf("invalid node signer key %s %s", hex.EncodeToString(tx.Extra), n.Signer)
		}
		if n.Payee.PublicSpendKey.String() == payeeSpend.String() {
			return fmt.Errorf("invalid node payee key %s %s", hex.EncodeToString(tx.Extra), n.Payee)
		}
	}

	// FIXME the node operation lock threshold should be optimized on pledging period
	return node.persistStore.AddNodeOperation(tx, timestamp, uint64(config.KernelNodePledgePeriodMinimum)*2)
}

func (node *Node) validateNodeCancelSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if tx.Asset != common.XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 2 {
		return fmt.Errorf("invalid outputs count %d for cancel transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for cancel transaction", len(tx.Inputs))
	}
	if len(tx.Extra) != len(crypto.Key{})*3 {
		return fmt.Errorf("invalid extra %s for cancel transaction", hex.EncodeToString(tx.Extra))
	}
	cancel, script := tx.Outputs[0], tx.Outputs[1]
	if cancel.Type != common.OutputTypeNodeCancel || script.Type != common.OutputTypeScript {
		return fmt.Errorf("invalid outputs type %d %d for cancel transaction", cancel.Type, script.Type)
	}
	if len(script.Keys) != 1 {
		return fmt.Errorf("invalid script output keys %d for cancel transaction", len(script.Keys))
	}
	if node.ConsensusPledging == nil {
		return fmt.Errorf("invalid consensus status")
	}
	if node.ConsensusPledging.Transaction != tx.Inputs[0].Hash {
		return fmt.Errorf("invalid plede utxo source %s %s", node.ConsensusPledging.Transaction, tx.Inputs[0].Hash)
	}

	pledge, _, err := node.persistStore.ReadTransaction(tx.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if len(pledge.Outputs) != 1 {
		return fmt.Errorf("invalid pledge utxo count %d", len(pledge.Outputs))
	}
	if pledge.Outputs[0].Type != common.OutputTypeNodePledge {
		return fmt.Errorf("invalid pledge utxo type %d", pledge.Outputs[0].Type)
	}
	if cancel.Amount.Cmp(pledge.Outputs[0].Amount.Div(100)) != 0 {
		return fmt.Errorf("invalid script output amount %s for cancel transaction", cancel.Amount)
	}
	pit, _, err := node.persistStore.ReadTransaction(pledge.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if pit == nil {
		return fmt.Errorf("invalid pledge input source %s:%d", pledge.Inputs[0].Hash, pledge.Inputs[0].Index)
	}
	pi := pit.Outputs[pledge.Inputs[0].Index]
	if len(pi.Keys) != 1 {
		return fmt.Errorf("invalid pledge input source keys %d", len(pi.Keys))
	}
	var a crypto.Key
	copy(a[:], tx.Extra[len(crypto.Key{})*2:])
	pledgeSpend := crypto.ViewGhostOutputKey(&pi.Keys[0], &a, &pi.Mask, uint64(pledge.Inputs[0].Index))
	targetSpend := crypto.ViewGhostOutputKey(&script.Keys[0], &a, &script.Mask, 1)
	if bytes.Compare(pledge.Extra, tx.Extra[:len(crypto.Key{})*2]) != 0 {
		return fmt.Errorf("invalid pledge and accpet key %s %s", hex.EncodeToString(pledge.Extra), hex.EncodeToString(tx.Extra))
	}
	if bytes.Compare(pledgeSpend[:], targetSpend[:]) != 0 {
		return fmt.Errorf("invalid pledge and cancel target %s %s", pledgeSpend, targetSpend)
	}

	timestamp := s.Timestamp
	if s.Timestamp == 0 && s.NodeId == node.IdForNetwork {
		timestamp = uint64(time.Now().UnixNano())
	}
	if timestamp < node.epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.epoch, timestamp)
	}

	since := timestamp - node.epoch
	hours := int(since / 3600000000000)
	if hours%24 < config.KernelNodeAcceptTimeBegin || hours%24 > config.KernelNodeAcceptTimeEnd {
		return fmt.Errorf("invalid node cancel hour %d", hours%24)
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if timestamp+threshold*2 < node.Graph.GraphTimestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Graph.GraphTimestamp, timestamp)
	}

	if timestamp < node.ConsensusPledging.Timestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.ConsensusPledging.Timestamp, timestamp)
	}
	elapse := time.Duration(timestamp - node.ConsensusPledging.Timestamp)
	if elapse < config.KernelNodeAcceptPeriodMinimum {
		return fmt.Errorf("invalid cancel period %d %d", config.KernelNodeAcceptPeriodMinimum, elapse)
	}
	if elapse > config.KernelNodeAcceptPeriodMaximum {
		return fmt.Errorf("invalid cancel period %d %d", config.KernelNodeAcceptPeriodMaximum, elapse)
	}

	// FIXME the node operation lock threshold should be optimized on pledging period
	return node.persistStore.AddNodeOperation(tx, timestamp, uint64(config.KernelNodePledgePeriodMinimum)*2)
}

func (node *Node) validateNodeAcceptSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if tx.Asset != common.XINAssetId {
		return fmt.Errorf("invalid node asset %s", tx.Asset.String())
	}
	if len(tx.Outputs) != 1 {
		return fmt.Errorf("invalid outputs count %d for accept transaction", len(tx.Outputs))
	}
	if len(tx.Inputs) != 1 {
		return fmt.Errorf("invalid inputs count %d for accept transaction", len(tx.Inputs))
	}
	if node.ConsensusPledging == nil {
		return fmt.Errorf("invalid consensus status")
	}
	if id := node.ConsensusPledging.Signer.Hash().ForNetwork(node.networkId); id != s.NodeId {
		return fmt.Errorf("invalid pledging node %s %s", id, s.NodeId)
	}
	if node.ConsensusPledging.Transaction != tx.Inputs[0].Hash {
		return fmt.Errorf("invalid plede utxo source %s %s", node.ConsensusPledging.Transaction, tx.Inputs[0].Hash)
	}

	pledge, _, err := node.persistStore.ReadTransaction(tx.Inputs[0].Hash)
	if err != nil {
		return err
	}
	if len(pledge.Outputs) != 1 {
		return fmt.Errorf("invalid pledge utxo count %d", len(pledge.Outputs))
	}
	if pledge.Outputs[0].Type != common.OutputTypeNodePledge {
		return fmt.Errorf("invalid pledge utxo type %d", pledge.Outputs[0].Type)
	}
	if bytes.Compare(pledge.Extra, tx.Extra) != 0 {
		return fmt.Errorf("invalid pledge and accpet key %s %s", hex.EncodeToString(pledge.Extra), hex.EncodeToString(tx.Extra))
	}

	timestamp := s.Timestamp
	if s.RoundNumber != 0 {
		return fmt.Errorf("invalid snapshot round %d", s.RoundNumber)
	}
	if s.Timestamp == 0 && s.NodeId == node.IdForNetwork {
		timestamp = uint64(time.Now().UnixNano())
	}
	if timestamp < node.epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.epoch, timestamp)
	}
	if r := node.Graph.CacheRound[s.NodeId]; r != nil {
		return fmt.Errorf("invalid graph round %s %d", s.NodeId, r.Number)
	}
	if r := node.Graph.FinalRound[s.NodeId]; r != nil {
		return fmt.Errorf("invalid graph round %s %d", s.NodeId, r.Number)
	}

	since := timestamp - node.epoch
	hours := int(since / 3600000000000)
	if hours%24 < config.KernelNodeAcceptTimeBegin || hours%24 > config.KernelNodeAcceptTimeEnd {
		return fmt.Errorf("invalid node accept hour %d", hours%24)
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if timestamp+threshold*2 < node.Graph.GraphTimestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Graph.GraphTimestamp, timestamp)
	}

	if timestamp < node.ConsensusPledging.Timestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.ConsensusPledging.Timestamp, timestamp)
	}
	elapse := time.Duration(timestamp - node.ConsensusPledging.Timestamp)
	if elapse < config.KernelNodeAcceptPeriodMinimum {
		return fmt.Errorf("invalid accept period %d %d", config.KernelNodeAcceptPeriodMinimum, elapse)
	}
	if elapse > config.KernelNodeAcceptPeriodMaximum {
		return fmt.Errorf("invalid accept period %d %d", config.KernelNodeAcceptPeriodMaximum, elapse)
	}

	return nil
}
