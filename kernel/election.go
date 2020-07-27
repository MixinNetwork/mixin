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
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
)

const (
	MainnetAcceptPeriodForkSnapshotHash = "b8855c19a38999f283d9be6daa45147aef47cc6d35007673f62390c2e137e4e1"
)

func (node *Node) ElectionLoop() {
	defer close(node.elc)

	ticker := time.NewTicker(time.Duration(node.custom.Node.KernelOprationPeriod) * time.Second)
	defer ticker.Stop()

	chain := node.GetOrCreateChain(node.IdForNetwork)
	for chain.State.CacheRound == nil {
		select {
		case <-node.done:
			return
		case <-ticker.C:
			now := uint64(clock.Now().UnixNano())
			if now < node.Epoch {
				logger.Printf("LOCAL TIME INVALID %d %d\n", now, node.Epoch)
				continue
			}
			hours := int((now-node.Epoch)/3600000000000) % 24
			if hours < config.KernelNodeAcceptTimeBegin || hours > config.KernelNodeAcceptTimeEnd {
				continue
			}

			err := node.tryToSendAcceptTransaction()
			if err != nil {
				logger.Println("tryToSendAcceptTransaction", err)
			}
		}
	}

	for {
		select {
		case <-node.done:
			return
		case <-ticker.C:
			candi, err := node.checkRemovePossibility(node.IdForNetwork, node.GraphTimestamp)
			if err != nil {
				logger.Printf("checkRemovePossibility %s", err.Error())
				continue
			}

			err = node.tryToSendRemoveTransaction(candi)
			if err != nil {
				logger.Println("tryToSendRemoveTransaction", err)
			}
		}
	}
}

func (node *Node) checkRemovePossibility(nodeId crypto.Hash, now uint64) (*common.Node, error) {
	if p := node.ConsensusPledging; p != nil {
		return nil, fmt.Errorf("still pledging now %s", p.Signer.String())
	}

	if now < node.Epoch {
		return nil, fmt.Errorf("local time invalid %d %d", now, node.Epoch)
	}
	hours := (now - node.Epoch) / 3600000000000
	if hours%24 < config.KernelNodeAcceptTimeBegin || hours%24 > config.KernelNodeAcceptTimeEnd {
		return nil, fmt.Errorf("invalid node remove hour %d", hours%24)
	}

	candi := node.AllNodesSorted[0]
	for _, cn := range node.AllNodesSorted {
		if cn.Timestamp == 0 {
			cn.Timestamp = node.Epoch
		}
		if now < cn.Timestamp {
			return nil, fmt.Errorf("invalid timestamp %d %d", cn.Timestamp, now)
		}
		elapse := time.Duration(now - cn.Timestamp)
		if elapse < config.KernelNodePledgePeriodMinimum {
			return nil, fmt.Errorf("invalid period %d %d %d %d", config.KernelNodePledgePeriodMinimum, elapse, now, cn.Timestamp)
		}
		if cn.State != common.NodeStateAccepted && cn.State != common.NodeStateCancelled && cn.State != common.NodeStateRemoved {
			return nil, fmt.Errorf("invalid node pending state %s %s", cn.Signer, cn.State)
		}
		if cn.State == common.NodeStateAccepted && candi.State != common.NodeStateAccepted {
			candi = cn
		}
	}
	if candi.State != common.NodeStateAccepted {
		return nil, fmt.Errorf("invalid node state %s %s", candi.IdForNetwork(node.networkId), candi.State)
	}

	days := int((now - node.Epoch) / 3600000000000 / 24)
	threshold := time.Duration(days/MintYearBatches*MintYearBatches) * 24 * time.Hour
	if t := node.Epoch + uint64(threshold); candi.Timestamp >= t {
		return nil, fmt.Errorf("all old nodes removed %d %d %d %d", candi.Timestamp, t, now, days)
	}

	if candi.IdForNetwork(node.networkId) == nodeId {
		return nil, fmt.Errorf("never handle the node remove transaction by the node self")
	}
	return candi, nil
}

func (node *Node) tryToSendRemoveTransaction(candi *common.Node) error {
	tx, err := node.buildRemoveTransaction(candi)
	if err != nil {
		return err
	}

	err = tx.Validate(node.persistStore)
	if err != nil {
		return err
	}
	err = node.persistStore.CachePutTransaction(tx)
	if err != nil {
		return err
	}
	chain := node.GetOrCreateChain(node.IdForNetwork)
	return chain.AppendCacheSnapshot(node.IdForNetwork, &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: tx.PayloadHash(),
	})
}

func (node *Node) buildRemoveTransaction(candi *common.Node) (*common.VersionedTransaction, error) {
	accept, _, err := node.persistStore.ReadTransaction(candi.Transaction)
	if err != nil {
		return nil, err
	}
	if accept == nil {
		return nil, fmt.Errorf("accept transaction not available yet %s", candi.Transaction)
	}
	if accept.PayloadHash() != candi.Transaction {
		return nil, fmt.Errorf("accept transaction malformed %s %s", candi.Transaction, accept.PayloadHash())
	}
	signer := candi.Signer.PublicSpendKey
	payee := candi.Payee.PublicSpendKey[:]
	if len(accept.Extra) != len(signer)*2 {
		return nil, fmt.Errorf("invalid accept transaction extra %s", hex.EncodeToString(accept.Extra))
	}
	if bytes.Compare(append(signer[:], payee...), accept.Extra) != 0 {
		return nil, fmt.Errorf("invalid accept transaction extra %s %s %s", hex.EncodeToString(accept.Extra), signer, hex.EncodeToString(payee))
	}

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddInput(candi.Transaction, 0)
	tx.Extra = accept.Extra
	script := common.NewThresholdScript(1)
	in := fmt.Sprintf("NODEREMOVE%s", candi.Signer.String())
	si := crypto.NewHash([]byte(candi.Payee.String() + in))
	seed := append(si[:], si[:]...)
	tx.AddOutputWithType(common.OutputTypeNodeRemove, []common.Address{candi.Payee}, script, accept.Outputs[0].Amount, seed)

	return tx.AsLatestVersion(), nil
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
	chain := node.GetOrCreateChain(node.IdForNetwork)
	err = chain.AppendCacheSnapshot(node.IdForNetwork, &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: ver.PayloadHash(),
	})
	logger.Println("tryToSendAcceptTransaction", ver.PayloadHash(), hex.EncodeToString(ver.Marshal()))
	return nil
}

func (node *Node) reloadConsensusNodesList(s *common.Snapshot, tx *common.VersionedTransaction) error {
	txType := tx.TransactionType()
	switch txType {
	case common.TransactionTypeNodePledge,
		common.TransactionTypeNodeCancel,
		common.TransactionTypeNodeAccept,
		common.TransactionTypeNodeResign,
		common.TransactionTypeNodeRemove:
		err := node.LoadConsensusNodes()
		if err != nil {
			return err
		}
	}
	if txType != common.TransactionTypeNodeAccept {
		return nil
	}
	chain := node.GetOrCreateChain(s.NodeId)
	return chain.loadState(node.networkId, node.AllNodesSorted)
}

func (node *Node) finalizeNodeAcceptSnapshot(s *common.Snapshot) error {
	cache := &CacheRound{
		NodeId:    s.NodeId,
		Number:    s.RoundNumber,
		Timestamp: s.Timestamp,
	}
	if err := cache.ValidateSnapshot(s, true); err != nil {
		panic("should never be here")
	}
	err := node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, cache.Timestamp)
	if err != nil {
		panic(err)
	}

	node.TopoWrite(s)

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

	chain := node.GetOrCreateChain(s.NodeId)
	chain.assignNewGraphRound(final, cache)
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
		timestamp = uint64(clock.Now().UnixNano())
	}

	if timestamp < node.Epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Epoch, timestamp)
	}
	since := timestamp - node.Epoch
	days := int(since / 3600000000000 / 24)
	elp := time.Duration((days%MintYearBatches)*24) * time.Hour
	eta := time.Duration((MintYearBatches-days%MintYearBatches)*24) * time.Hour
	if eta < config.KernelNodeAcceptPeriodMaximum*2 || elp < config.KernelNodeAcceptPeriodMinimum*2 {
		return fmt.Errorf("invalid pledge timestamp %d %d", eta, elp)
	}

	var signerSpend crypto.Key
	copy(signerSpend[:], tx.Extra)
	for _, cn := range node.persistStore.ReadAllNodes() {
		if cn.Timestamp == 0 {
			cn.Timestamp = node.Epoch
		}
		if timestamp < cn.Timestamp {
			return fmt.Errorf("invalid snapshot timestamp %d %d", cn.Timestamp, timestamp)
		}
		elapse := time.Duration(timestamp - cn.Timestamp)
		if elapse < config.KernelNodePledgePeriodMinimum {
			return fmt.Errorf("invalid pledge period %d %d", config.KernelNodePledgePeriodMinimum, elapse)
		}
		if cn.State != common.NodeStateAccepted && cn.State != common.NodeStateCancelled && cn.State != common.NodeStateRemoved {
			return fmt.Errorf("invalid node pending state %s %s", cn.Signer, cn.State)
		}
		if cn.Signer.PublicSpendKey.String() == signerSpend.String() {
			return fmt.Errorf("invalid node signer key %s %s", hex.EncodeToString(tx.Extra), cn.Signer)
		}
		if cn.Payee.PublicSpendKey.String() == signerSpend.String() {
			return fmt.Errorf("invalid node signer key %s %s", hex.EncodeToString(tx.Extra), cn.Payee)
		}
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

	// FIXME the node operation lock threshold should be optimized on pledging period
	return node.persistStore.AddNodeOperation(tx, timestamp, uint64(config.KernelNodePledgePeriodMinimum)*2)
}

func (node *Node) validateNodeCancelSnapshot(s *common.Snapshot, tx *common.VersionedTransaction, finalized bool) error {
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
		timestamp = uint64(clock.Now().UnixNano())
	}
	if timestamp < node.Epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Epoch, timestamp)
	}

	since := timestamp - node.Epoch
	hours := int(since / 3600000000000)
	if hours%24 < config.KernelNodeAcceptTimeBegin || hours%24 > config.KernelNodeAcceptTimeEnd {
		return fmt.Errorf("invalid node cancel hour %d", hours%24)
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if !finalized && timestamp+threshold*2 < node.GraphTimestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.GraphTimestamp, timestamp)
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

func (node *Node) validateNodeRemoveSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	timestamp := s.Timestamp
	if s.Timestamp == 0 && s.NodeId == node.IdForNetwork {
		timestamp = uint64(clock.Now().UnixNano())
	}
	candi, err := node.checkRemovePossibility(s.NodeId, timestamp)
	if err != nil {
		return err
	}
	cantx, err := node.buildRemoveTransaction(candi)
	if err != nil {
		return err
	}
	if cantx.PayloadHash() != tx.PayloadHash() {
		return fmt.Errorf("invalid node remove transaction %s %s", cantx.PayloadHash(), tx.PayloadHash())
	}
	return nil
}

func (node *Node) validateNodeAcceptSnapshot(s *common.Snapshot, tx *common.VersionedTransaction, finalized bool) error {
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
	if id := node.ConsensusPledging.IdForNetwork(node.networkId); id != s.NodeId {
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
		timestamp = uint64(clock.Now().UnixNano())
	}
	if timestamp < node.Epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Epoch, timestamp)
	}
	chain := node.GetOrCreateChain(s.NodeId)
	if r := chain.State.CacheRound; r != nil {
		return fmt.Errorf("invalid graph round %s %d", s.NodeId, r.Number)
	}
	if r := chain.State.FinalRound; r != nil {
		return fmt.Errorf("invalid graph round %s %d", s.NodeId, r.Number)
	}

	since := timestamp - node.Epoch
	hours := int(since / 3600000000000)
	if hours%24 < config.KernelNodeAcceptTimeBegin || hours%24 > config.KernelNodeAcceptTimeEnd {
		return fmt.Errorf("invalid node accept hour %d", hours%24)
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if !finalized && timestamp+threshold*2 < node.GraphTimestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.GraphTimestamp, timestamp)
	}

	if timestamp < node.ConsensusPledging.Timestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.ConsensusPledging.Timestamp, timestamp)
	}
	elapse := time.Duration(timestamp - node.ConsensusPledging.Timestamp)
	if elapse < config.KernelNodeAcceptPeriodMinimum {
		if s.PayloadHash().String() == MainnetAcceptPeriodForkSnapshotHash {
			logger.Printf("FORK invalid accept period %d %d\n", config.KernelNodeAcceptPeriodMinimum, elapse)
		} else {
			return fmt.Errorf("invalid accept period %d %d", config.KernelNodeAcceptPeriodMinimum, elapse)
		}
	}
	if elapse > config.KernelNodeAcceptPeriodMaximum {
		return fmt.Errorf("invalid accept period %d %d", config.KernelNodeAcceptPeriodMaximum, elapse)
	}

	return nil
}
