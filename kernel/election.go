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

var (
	MainnetRollingRemovalForkTransactionMap = map[string]bool{
		"d5af53561d99eb52af2b98b57d3fb0cc8ae4c6449ec6c89d8427201051a947a2": true,
		"aef48f91a3d6ffebc2dd0178d47de66cee222e48827adbf339d4197d5eee8af9": true,
		"f77436fd09c2248b79a8f54321e0332d247af489b26d4a4216d8eeb3596e8d4b": true,
		"b1ccc15b4e6c97e1d41ccaffc2c933368c03c9d0ef80c45f9fa41d013b23be22": true,
		"e369448593bdb04c1ab9dd9d6536ea6ac91a49e1b8fdb8374ea1d896034267bc": true,
		"6182da6d3e7bcee9d7a215edc04015aac1c6a9d4a84cef34e6c4fcbbd8d6cadf": true,
		"5447772e29a35487fc42e6d10ba2b7ea6a7d77f99181b8a6f7ae25e964ff0994": true,
		"246de39853bcffeb885aa27df9e6df0e19ddfcee1967b29c2c81e86b386affde": true,
		"04f7ba291b44f838e8e784e76561455e9f068c0dedd750870e16169cdfb6a660": true,
		"3d3f223aaeab0bc54ded3420c9949b1c871d1b0e245c3f53362cf99ccaadf337": true,
		"c3c46410adfd1ebf8a3753d5d685fffd31a3c72c62118a678731e6292b2a426d": true,
		"b26b3accf232512924087fc810a3ace700d8ccfd75a392e7403471465bc1a886": true,
		"1c5883bc30f0caec912cc94011aa4ade2131cd63d21e652fdc8e49d62d79d73f": true,
		"356c9511de0a621f87cb6c98be7bc8ace90a7c8021ea02ba7cfe71f94c8348c3": true,
		"86cbbefd4b1a4ebf84fa6c7429c278032bf79cef0ce00ec0bb4c7bbb081dde72": true,
		"ad1d3884c9335580ccea6cfb2a66cfb95f9bb77431cf5fda80c66028d796963e": true,
		"46001500b12a3247e4a00fb32ac42f865f8bf320e01f55eee76aefe898b1cbb6": true,
		"4a0ddc369fe4cf60118bb5dc58729841c356c807ca9cc6c2cc62516576d65fb2": true,
		"98bcb9acfebcbd666a423f9f4628a2946ce1939e9f3ba5653270774686d6df1b": true,
		"7fa19dbf5c014d37485412d90b2d60e14b4778c969c0b5da253d2538795cb0e3": true,
		"35ba9f06bcf68ffab52d4fddfab6be11a7eddd8cf94baf10500a289ea97031af": true,
		"46ae3d3d5c173f0b691250d7a3b24ba02731d7b9eae1808c655c0ca031b70cb6": true,
		"ba7c57177d12c7a598bb1ac5ffc1c0ac52926f170da6baf438098b607d15f5c1": true,
		"23e5e0b13eec7413116011b78a1a2bac0bc2070f02a6999d69a5c604e555b9b1": true,
		"3e85d0329530a04c0132cb69c50b59103e7db405865de3dce41854d203778184": true,
		"c3918ece3f938448e2a573ec88b0a5cedd2449d6fb2af21804a1dd24fa9b4c29": true,
		"65da3f839b795bd57a52638767621ec9bc764b929a23ca26ebfc5cf49686b28e": true,
		"a133c2b154e8103b39bca963acb7f545838e06f784dcfaa761fc6ef2163b850e": true,
		"494a9b4326ffb2d22e53cd62945a349b2c205a2a1f3288ca8bee47446e535af8": true,
		"b99fb0d60318c48d793840700789009ff34ff4632e788a7e71138bdae4772d59": true,
		"0213977d3c00a91de68904fb03ce3982e139200a2ce2e6f5332c9c3fb83743c5": true,
		"d598c36ed84b4318dffbeb81efac93be2bfd22a76f5099eef8e6a5b508628a8a": true,
	}
)

func (node *Node) ElectionLoop() {
	defer close(node.elc)

	ticker := time.NewTicker(time.Duration(node.custom.Node.KernelOprationPeriod) * time.Second)
	defer ticker.Stop()

	chain := node.GetOrCreateChain(node.IdForNetwork)
	for chain.State == nil {
		select {
		case <-node.done:
			return
		case <-ticker.C:
			err := chain.tryToSendAcceptTransaction()
			if err != nil {
				logger.Println("tryToSendAcceptTransaction", err)
			}
		}
	}
	logger.Println("ElectionLoop ACCEPTED!")

	for {
		select {
		case <-node.done:
			return
		case <-ticker.C:
			err := node.tryToSendRemoveTransaction()
			if err != nil {
				logger.Println("tryToSendRemoveTransaction", err)
			}
		}
	}
}

func (node *Node) checkRemovePossibility(nodeId crypto.Hash, now uint64, old *common.VersionedTransaction) (*CNode, error) {
	if p := node.PledgingNode(now); p != nil {
		return nil, fmt.Errorf("still pledging now %s", p.Signer.String())
	}

	if now < node.Epoch {
		return nil, fmt.Errorf("local time invalid %d %d", now, node.Epoch)
	}
	hours := (now - node.Epoch) / 3600000000000
	if hours%24 < config.KernelNodeAcceptTimeBegin || hours%24 > config.KernelNodeAcceptTimeEnd {
		return nil, fmt.Errorf("invalid node remove hour %d", hours%24)
	}

	var candi *CNode
	var accepted []*CNode
	for _, cn := range node.NodesListWithoutState(now, false) {
		if old != nil && cn.Transaction == old.PayloadHash() {
			candi = cn
			continue
		}
		if now < cn.Timestamp {
			return nil, fmt.Errorf("invalid timestamp %d %d", cn.Timestamp, now)
		}
		elapse := time.Duration(now - cn.Timestamp)
		if elapse < config.KernelNodePledgePeriodMinimum {
			return nil, fmt.Errorf("invalid period %d %d %d %d", config.KernelNodePledgePeriodMinimum, elapse, now, cn.Timestamp)
		}
		switch cn.State {
		case common.NodeStateAccepted:
			accepted = append(accepted, cn)
		case common.NodeStateCancelled:
		case common.NodeStateRemoved:
		default:
			return nil, fmt.Errorf("invalid node pending state %s %s", cn.Signer, cn.State)
		}
	}
	if len(accepted) <= config.KernelMinimumNodesCount {
		return nil, fmt.Errorf("all old nodes removed %d", len(accepted))
	}
	if candi == nil {
		candi = accepted[0]
	}
	if candi.IdForNetwork == nodeId {
		return nil, fmt.Errorf("never handle the node remove transaction by the node self")
	}
	return candi, nil
}

func (node *Node) buildNodeRemoveTransaction(nodeId crypto.Hash, timestamp uint64, old *common.VersionedTransaction) (*common.VersionedTransaction, error) {
	candi, err := node.checkRemovePossibility(nodeId, timestamp, old)
	if err != nil {
		return nil, err
	}
	if old != nil && candi.Transaction == old.PayloadHash() {
		return old, nil
	}

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
	if !bytes.Equal(append(signer[:], payee...), accept.Extra) {
		return nil, fmt.Errorf("invalid accept transaction extra %s %s %s", hex.EncodeToString(accept.Extra), signer, hex.EncodeToString(payee))
	}

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddInput(candi.Transaction, 0)
	tx.Extra = accept.Extra
	script := common.NewThresholdScript(1)
	in := fmt.Sprintf("NODEREMOVE%s", candi.Signer.String())
	si := crypto.NewHash([]byte(candi.Payee.String() + in))
	seed := append(si[:], si[:]...)
	tx.AddOutputWithType(common.OutputTypeNodeRemove, []*common.Address{&candi.Payee}, script, accept.Outputs[0].Amount, seed)

	ver := tx.AsLatestVersion()
	fork := uint64(ElectionTransactionV2ForkHack.UnixNano())
	if node.networkId.String() == config.MainnetId && timestamp < fork {
		ver.Version = 1
	}
	return ver, nil
}

func (node *Node) tryToSendRemoveTransaction() error {
	tx, err := node.buildNodeRemoveTransaction(node.IdForNetwork, node.GraphTimestamp, nil)
	if err != nil {
		return err
	}
	logger.Verbosef("tryToSendRemoveTransaction %s\n", tx.PayloadHash())

	err = tx.Validate(node.persistStore, false)
	if err != nil {
		return err
	}
	err = node.persistStore.CachePutTransaction(tx)
	if err != nil {
		return err
	}
	chain := node.GetOrCreateChain(node.IdForNetwork)
	return chain.AppendSelfEmpty(&common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: tx.PayloadHash(),
	})
}

func (node *Node) validateNodeRemoveSnapshot(s *common.Snapshot, tx *common.VersionedTransaction) error {
	if node.networkId.String() == config.MainnetId && MainnetRollingRemovalForkTransactionMap[tx.PayloadHash().String()] {
		return nil
	}
	timestamp := s.Timestamp
	if s.Timestamp == 0 && s.NodeId == node.IdForNetwork {
		timestamp = uint64(clock.Now().UnixNano())
	}
	cantx, err := node.buildNodeRemoveTransaction(s.NodeId, timestamp, tx)
	if err != nil {
		return err
	}
	if cantx.PayloadHash() != tx.PayloadHash() {
		return fmt.Errorf("invalid node remove transaction %s %s", cantx.PayloadHash(), tx.PayloadHash())
	}
	return nil
}

func (chain *Chain) checkNodeAcceptPossibility(timestamp uint64, s *common.Snapshot, finalized bool) error {
	ci, epoch := chain.ConsensusInfo, chain.node.Epoch
	if chain.State != nil {
		return fmt.Errorf("invalid graph round %s %d", chain.ChainId, chain.State.CacheRound.Number)
	}

	pledging := chain.node.PledgingNode(timestamp)
	if pledging == nil {
		return fmt.Errorf("no consensus pledging node %t", pledging == nil)
	}
	if pledging.Signer.String() != ci.Signer.String() {
		return fmt.Errorf("invalid consensus pledging node %s %s", pledging.Signer, ci.Signer)
	}

	if timestamp < epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", epoch, timestamp)
	}

	since := timestamp - epoch
	hours := int(since / 3600000000000)
	if hours%24 < config.KernelNodeAcceptTimeBegin || hours%24 > config.KernelNodeAcceptTimeEnd {
		return fmt.Errorf("invalid node accept hour %d", hours%24)
	}

	threshold := config.SnapshotRoundGap * config.SnapshotReferenceThreshold
	if !finalized && timestamp+threshold*2 < chain.node.GraphTimestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", chain.node.GraphTimestamp, timestamp)
	}

	if timestamp < pledging.Timestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", pledging.Timestamp, timestamp)
	}
	elapse := time.Duration(timestamp - pledging.Timestamp)
	if elapse < config.KernelNodeAcceptPeriodMinimum {
		if finalized && s.PayloadHash().String() == MainnetAcceptPeriodForkSnapshotHash {
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

func (chain *Chain) buildNodeAcceptTransaction(timestamp uint64, s *common.Snapshot, finalized bool) (*common.VersionedTransaction, error) {
	err := chain.checkNodeAcceptPossibility(timestamp, s, finalized)
	if err != nil {
		return nil, err
	}

	ci := chain.ConsensusInfo
	pledge, _, err := chain.node.persistStore.ReadTransaction(ci.Transaction)
	if err != nil {
		return nil, err
	}
	if pledge == nil {
		return nil, fmt.Errorf("pledge transaction not available yet %s", ci.Transaction)
	}
	if pledge.PayloadHash() != ci.Transaction {
		return nil, fmt.Errorf("pledge transaction malformed %s %s", ci.Transaction, pledge.PayloadHash())
	}
	signer := ci.Signer.PublicSpendKey
	if len(pledge.Extra) != len(signer)*2 {
		return nil, fmt.Errorf("invalid pledge transaction extra %s", hex.EncodeToString(pledge.Extra))
	}
	if !bytes.Equal(signer[:], pledge.Extra[:len(signer)]) {
		return nil, fmt.Errorf("invalid pledge transaction extra %s %s", hex.EncodeToString(pledge.Extra[:len(signer)]), signer)
	}

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddInput(ci.Transaction, 0)
	tx.AddOutputWithType(common.OutputTypeNodeAccept, nil, common.Script{}, pledge.Outputs[0].Amount, []byte{})
	tx.Extra = pledge.Extra

	ver := tx.AsLatestVersion()
	fork := uint64(ElectionTransactionV2ForkHack.UnixNano())
	if chain.node.networkId.String() == config.MainnetId && timestamp < fork {
		ver.Version = 1
	}
	return ver, nil
}

func (chain *Chain) tryToSendAcceptTransaction() error {
	ver, err := chain.buildNodeAcceptTransaction(uint64(clock.Now().UnixNano()), nil, false)
	if err != nil {
		return err
	}
	logger.Verbosef("tryToSendAcceptTransaction %s\n", ver.PayloadHash())

	err = ver.Validate(chain.node.persistStore, false)
	if err != nil {
		return err
	}
	err = chain.node.persistStore.CachePutTransaction(ver)
	if err != nil {
		return err
	}
	chain.AppendSelfEmpty(&common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      chain.ChainId,
		Transaction: ver.PayloadHash(),
	})
	logger.Println("tryToSendAcceptTransaction", ver.PayloadHash(), hex.EncodeToString(ver.Marshal()))
	return nil
}

func (node *Node) validateNodeAcceptSnapshot(s *common.Snapshot, tx *common.VersionedTransaction, finalized bool) error {
	timestamp := s.Timestamp
	if timestamp == 0 && s.NodeId == node.IdForNetwork {
		timestamp = uint64(clock.Now().UnixNano())
	}
	if s.RoundNumber != 0 {
		return fmt.Errorf("invalid snapshot round %d", s.RoundNumber)
	}

	chain := node.GetOrCreateChain(s.NodeId)
	ver, err := chain.buildNodeAcceptTransaction(timestamp, s, finalized)
	if err != nil {
		return err
	}
	if ver.PayloadHash() != tx.PayloadHash() {
		return fmt.Errorf("invalid node accept transaction %s %s", ver.PayloadHash(), tx.PayloadHash())
	}

	return nil
}

func (node *Node) reloadConsensusNodesList(s *common.Snapshot, tx *common.VersionedTransaction) error {
	switch tx.TransactionType() {
	case common.TransactionTypeNodePledge,
		common.TransactionTypeNodeCancel,
		common.TransactionTypeNodeAccept,
		common.TransactionTypeNodeRemove:
	default:
		return nil
	}
	logger.Printf("reloadConsensusNodesList(%v, %v)\n", s, tx)
	err := node.LoadConsensusNodes()
	if err != nil {
		return err
	}

	chain := node.GetOrCreateChain(s.NodeId)
	err = chain.loadState()
	if err != nil {
		return err
	}
	if chain.ConsensusInfo == nil {
		panic("should never be here")
	}

	var signer common.Address
	copy(signer.PublicSpendKey[:], tx.Extra)
	signer.PrivateViewKey = signer.PublicSpendKey.DeterministicHashDerive()
	signer.PublicViewKey = signer.PrivateViewKey.Public()
	id := signer.Hash().ForNetwork(node.networkId)
	if id == s.NodeId {
		return nil
	}

	chain = node.GetOrCreateChain(id)
	err = chain.loadState()
	if err != nil {
		return err
	}
	if chain.ConsensusInfo == nil {
		panic("should never be here")
	}
	return nil
}

func (node *Node) finalizeNodeAcceptSnapshot(s *common.Snapshot, signers []crypto.Hash) error {
	logger.Printf("finalizeNodeAcceptSnapshot(%v)\n", s)
	cache := &CacheRound{
		NodeId:    s.NodeId,
		Number:    s.RoundNumber,
		Timestamp: s.Timestamp,
	}
	if err := cache.validateSnapshot(s, true); err != nil {
		panic("should never be here")
	}
	err := node.persistStore.StartNewRound(cache.NodeId, cache.Number, cache.References, cache.Timestamp)
	if err != nil {
		panic(err)
	}

	node.TopoWrite(s, signers)

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
	if err := chain.loadState(); err != nil {
		return err
	}
	chain.StepForward()
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

	var signerSpend crypto.Key
	copy(signerSpend[:], tx.Extra)
	offset := timestamp + uint64(config.KernelNodePledgePeriodMinimum)
	for _, cn := range node.NodesListWithoutState(offset, false) {
		if cn.Transaction == tx.PayloadHash() {
			continue
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
	if tx.Outputs[0].Amount.Cmp(pledgeAmount(time.Duration(since))) != 0 {
		return fmt.Errorf("invalid pledge amount %s", tx.Outputs[0].Amount.String())
	}

	// FIXME the node operation lock threshold should be optimized on pledging period
	return node.persistStore.AddNodeOperation(tx, timestamp, uint64(config.KernelNodePledgePeriodMinimum)*2)
}

func (node *Node) validateNodeCancelSnapshot(s *common.Snapshot, tx *common.VersionedTransaction, finalized bool) error {
	timestamp := s.Timestamp
	if s.Timestamp == 0 && s.NodeId == node.IdForNetwork {
		timestamp = uint64(clock.Now().UnixNano())
	}
	if timestamp < node.Epoch {
		return fmt.Errorf("invalid snapshot timestamp %d %d", node.Epoch, timestamp)
	}

	pledging := node.PledgingNode(timestamp)
	if pledging == nil {
		return fmt.Errorf("invalid consensus status")
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

	if timestamp < pledging.Timestamp {
		return fmt.Errorf("invalid snapshot timestamp %d %d", pledging.Timestamp, timestamp)
	}
	elapse := time.Duration(timestamp - pledging.Timestamp)
	if elapse < config.KernelNodeAcceptPeriodMinimum {
		return fmt.Errorf("invalid cancel period %d %d", config.KernelNodeAcceptPeriodMinimum, elapse)
	}
	if elapse > config.KernelNodeAcceptPeriodMaximum {
		return fmt.Errorf("invalid cancel period %d %d", config.KernelNodeAcceptPeriodMaximum, elapse)
	}

	// FIXME the node operation lock threshold should be optimized on pledging period
	return node.persistStore.AddNodeOperation(tx, timestamp, uint64(config.KernelNodePledgePeriodMinimum)*2)
}
