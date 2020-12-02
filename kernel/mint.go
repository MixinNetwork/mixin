package kernel

import (
	"fmt"
	"sort"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
)

const (
	MainnetMintPeriodForkBatch     = 72
	MainnetMintPeriodForkTimeBegin = 6
	MainnetMintPeriodForkTimeEnd   = 18
)

var (
	MintPool        common.Integer
	MintLiquidity   common.Integer
	MintYearShares  int
	MintYearBatches int
	MintNodeMaximum int
)

func init() {
	MintPool = common.NewInteger(500000)
	MintLiquidity = common.NewInteger(500000)
	MintYearShares = 10
	MintYearBatches = 365
	MintNodeMaximum = 50
}

func (node *Node) MintLoop() {
	defer close(node.mlc)

	ticker := time.NewTicker(time.Duration(node.custom.Node.KernelOprationPeriod) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-node.done:
			return
		case <-ticker.C:
			err := node.tryToMintKernelNode()
			if err != nil {
				logger.Println(node.IdForNetwork, "tryToMintKernelNode", err)
			}
		}
	}
}

func (node *Node) PoolSize() (common.Integer, error) {
	dist, err := node.persistStore.ReadLastMintDistribution(common.MintGroupKernelNode)
	if err != nil {
		return common.Zero, err
	}
	return poolSize(int(dist.Batch)), nil
}

func poolSize(batch int) common.Integer {
	mint, pool := common.Zero, MintPool
	for i := 0; i < batch/MintYearBatches; i++ {
		year := pool.Div(MintYearShares)
		mint = mint.Add(year.Div(10).Mul(9))
		pool = pool.Sub(year)
	}
	day := pool.Div(MintYearShares).Div(MintYearBatches)
	if count := batch % MintYearBatches; count > 0 {
		mint = mint.Add(day.Div(10).Mul(9).Mul(count))
	}
	if mint.Sign() > 0 {
		return MintPool.Sub(mint)
	}
	return MintPool
}

func pledgeAmount(sinceEpoch time.Duration) common.Integer {
	batch := int(sinceEpoch / 3600000000000 / 24)
	liquidity, pool := MintLiquidity, MintPool
	for i := 0; i < batch/MintYearBatches; i++ {
		share := pool.Div(MintYearShares)
		liquidity = liquidity.Add(share)
		pool = pool.Sub(share)
	}
	return liquidity.Div(MintNodeMaximum)
}

func (node *Node) buildMintTransaction(timestamp uint64, validateOnly bool) *common.VersionedTransaction {
	batch, amount := node.checkMintPossibility(timestamp, validateOnly)
	if amount.Sign() <= 0 || batch <= 0 {
		return nil
	}

	nodes := node.sortMintNodes(timestamp)
	per := amount.Div(len(nodes))
	diff := amount.Sub(per.Mul(len(nodes)))

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddKernelNodeMintInput(uint64(batch), amount)
	script := common.NewThresholdScript(1)
	for _, n := range nodes {
		in := fmt.Sprintf("MINTKERNELNODE%d", batch)
		si := crypto.NewHash([]byte(n.Signer.String() + in))
		seed := append(si[:], si[:]...)
		tx.AddScriptOutput([]common.Address{n.Payee}, script, per, seed)
	}

	if diff.Sign() > 0 {
		addr := common.NewAddressFromSeed(make([]byte, 64))
		script := common.NewThresholdScript(common.Operator64)
		in := fmt.Sprintf("MINTKERNELNODE%dDIFF", batch)
		si := crypto.NewHash([]byte(addr.String() + in))
		seed := append(si[:], si[:]...)
		tx.AddScriptOutput([]common.Address{addr}, script, diff, seed)
	}

	return tx.AsLatestVersion()
}

func (node *Node) tryToMintKernelNode() error {
	signed := node.buildMintTransaction(node.GraphTimestamp, false)
	if signed == nil {
		return nil
	}

	err := signed.SignInput(node.persistStore, 0, []common.Address{node.Signer})
	if err != nil {
		return err
	}
	err = signed.Validate(node.persistStore)
	if err != nil {
		return err
	}
	err = node.persistStore.CachePutTransaction(signed)
	if err != nil {
		return err
	}
	chain := node.GetOrCreateChain(node.IdForNetwork)
	return chain.AppendSelfEmpty(&common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: signed.PayloadHash(),
	})
}

func (node *Node) validateMintSnapshot(snap *common.Snapshot, tx *common.VersionedTransaction) error {
	timestamp := snap.Timestamp
	if snap.Timestamp == 0 && snap.NodeId == node.IdForNetwork {
		timestamp = uint64(clock.Now().UnixNano())
	}
	signed := node.buildMintTransaction(timestamp, true)
	if signed == nil {
		return fmt.Errorf("no mint available at %d", timestamp)
	}

	if tx.PayloadHash() != signed.PayloadHash() {
		return fmt.Errorf("malformed mint transaction at %d", timestamp)
	}
	return nil
}

func (node *Node) checkMintPossibility(timestamp uint64, validateOnly bool) (int, common.Integer) {
	if timestamp <= node.Epoch {
		return 0, common.Zero
	}

	since := timestamp - node.Epoch
	hours := int(since / 3600000000000)
	batch := hours / 24
	if batch < 1 {
		return 0, common.Zero
	}
	kmb, kme := config.KernelMintTimeBegin, config.KernelMintTimeEnd
	if node.networkId.String() == config.MainnetId && batch < MainnetMintPeriodForkBatch {
		kmb = MainnetMintPeriodForkTimeBegin
		kme = MainnetMintPeriodForkTimeEnd
	}
	if hours%24 < kmb || hours%24 > kme {
		return 0, common.Zero
	}

	pool := MintPool
	for i := 0; i < batch/MintYearBatches; i++ {
		pool = pool.Sub(pool.Div(MintYearShares))
	}
	pool = pool.Div(MintYearShares)
	total := pool.Div(MintYearBatches)
	light := total.Div(10)
	full := light.Mul(9)

	dist, err := node.persistStore.ReadLastMintDistribution(common.MintGroupKernelNode)
	if err != nil {
		logger.Verbosef("ReadLastMintDistribution ERROR %s\n", err)
		return 0, common.Zero
	}
	logger.Verbosef("checkMintPossibility OLD %s %s %s %s %d %s %d\n", pool, total, light, full, batch, dist.Amount, dist.Batch)

	if batch < int(dist.Batch) {
		return 0, common.Zero
	}
	if batch == int(dist.Batch) {
		if validateOnly {
			return batch, dist.Amount
		}
		return 0, common.Zero
	}

	amount := full.Mul(batch - int(dist.Batch))
	logger.Verbosef("checkMintPossibility NEW %s %s %s %s %s %d %s %d\n", pool, total, light, full, amount, batch, dist.Amount, dist.Batch)
	return batch, amount
}

func (node *Node) sortMintNodes(timestamp uint64) []*CNode {
	accepted := node.NodesListWithoutState(timestamp, true)
	sort.Slice(accepted, func(i, j int) bool {
		a := accepted[i].IdForNetwork
		b := accepted[j].IdForNetwork
		return a.String() < b.String()
	})
	return accepted
}

func (node *Node) NodesListWithoutState(threshold uint64, acceptedOnly bool) []*CNode {
	filter := make(map[crypto.Hash]*CNode)
	for _, n := range node.allNodesSortedWithState {
		if n.Timestamp >= threshold {
			break
		}
		filter[n.IdForNetwork] = n
	}
	nodes := make([]*CNode, 0)
	for _, n := range filter {
		if !acceptedOnly || n.State == common.NodeStateAccepted {
			nodes = append(nodes, n)
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Timestamp < nodes[j].Timestamp {
			return true
		}
		if nodes[i].Timestamp > nodes[j].Timestamp {
			return false
		}
		a := nodes[i].IdForNetwork
		b := nodes[j].IdForNetwork
		return a.String() < b.String()
	})
	for index, i := 0, 0; i < len(nodes); i++ {
		cn := nodes[i]
		cn.ConsensusIndex = index
		switch cn.State {
		case common.NodeStateAccepted, common.NodeStatePledging:
			index++
		}
	}
	return nodes
}
