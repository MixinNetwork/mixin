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

func (node *Node) MintLoop() error {
	for {
		time.Sleep(7 * time.Minute)

		batch, amount := node.checkMintPossibility(node.Graph.GraphTimestamp, false)
		if amount.Sign() <= 0 || batch <= 0 {
			continue
		}

		err := node.tryToMintKernelNode(uint64(batch), amount)
		if err != nil {
			logger.Println(node.IdForNetwork, "tryToMintKernelNode", err)
		}
	}
	return nil
}

func (node *Node) PoolSize() (common.Integer, error) {
	dist, err := node.persistStore.ReadLastMintDistribution(common.MintGroupKernelNode)
	if err != nil {
		return common.Zero, err
	}
	mint, pool := common.Zero, MintPool
	for i := 0; i < int(dist.Batch)/MintYearBatches; i++ {
		year := pool.Div(MintYearShares)
		mint = mint.Add(year.Div(10).Mul(9))
		pool = pool.Sub(year)
	}
	day := pool.Div(MintYearShares).Div(MintYearBatches)
	mint = mint.Add(day.Div(10).Mul(9).Mul(int(dist.Batch) % MintYearBatches))
	return MintPool.Sub(mint), nil
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

func (node *Node) tryToMintKernelNode(batch uint64, amount common.Integer) error {
	nodes := node.sortMintNodes()
	per := amount.Div(len(nodes))
	diff := amount.Sub(per.Mul(len(nodes)))

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddKernelNodeMintInput(batch, amount)
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

	signed := tx.AsLatestVersion()
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
	return node.QueueAppendSnapshot(node.IdForNetwork, &common.Snapshot{
		Version:     common.SnapshotVersion,
		NodeId:      node.IdForNetwork,
		Transaction: signed.PayloadHash(),
	}, false)
}

func (node *Node) validateMintSnapshot(snap *common.Snapshot, tx *common.VersionedTransaction) error {
	timestamp := snap.Timestamp
	if snap.Timestamp == 0 && snap.NodeId == node.IdForNetwork {
		timestamp = uint64(clock.Now().UnixNano())
	}
	batch, amount := node.checkMintPossibility(timestamp, true)
	if amount.Sign() <= 0 || batch <= 0 {
		return fmt.Errorf("no mint available %d %s", batch, amount.String())
	}
	mint := tx.Inputs[0].Mint
	if mint.Batch != uint64(batch) && mint.Amount.Cmp(amount) != 0 {
		return fmt.Errorf("invalid mint data %d %s", batch, amount.String())
	}

	nodes := node.sortMintNodes()
	per := amount.Div(len(nodes))
	diff := amount.Sub(per.Mul(len(nodes)))

	if diff.Sign() > 0 {
		if len(nodes)+1 != len(tx.Outputs) {
			return fmt.Errorf("invalid mint outputs count with diff %d %d", len(nodes), len(tx.Outputs))
		}
		out := tx.Outputs[len(nodes)]
		if diff.Cmp(out.Amount) != 0 {
			return fmt.Errorf("invalid mint diff %s", diff.String())
		}
		if out.Type != common.OutputTypeScript {
			return fmt.Errorf("invalid mint diff type %d", out.Type)
		}
		if out.Script.String() != common.NewThresholdScript(common.Operator64).String() {
			return fmt.Errorf("invalid mint diff script %s", out.Script.String())
		}
		if len(out.Keys) != 1 {
			return fmt.Errorf("invalid mint diff keys %d", len(out.Keys))
		}
		addr := common.NewAddressFromSeed(make([]byte, 64))
		in := fmt.Sprintf("MINTKERNELNODE%dDIFF", mint.Batch)
		seed := crypto.NewHash([]byte(addr.String() + in))
		r := crypto.NewKeyFromSeed(append(seed[:], seed[:]...))
		if r.Public() != out.Mask {
			return fmt.Errorf("invalid mint diff mask %s %s", r.Public().String(), out.Mask.String())
		}
		ghost := crypto.ViewGhostOutputKey(&out.Keys[0], &addr.PrivateViewKey, &out.Mask, uint64(len(nodes)))
		if *ghost != addr.PublicSpendKey {
			return fmt.Errorf("invalid mint diff signature %s %s", addr.PublicSpendKey.String(), ghost.String())
		}
		return nil
	} else if len(nodes) != len(tx.Outputs) {
		return fmt.Errorf("invalid mint outputs count %d %d", len(nodes), len(tx.Outputs))
	}

	for i, out := range tx.Outputs {
		if i == len(nodes) {
			break
		}
		if out.Type != common.OutputTypeScript {
			return fmt.Errorf("invalid mint output type %d", out.Type)
		}
		if per.Cmp(out.Amount) != 0 {
			return fmt.Errorf("invalid mint output amount %s %s", per.String(), out.Amount.String())
		}
		if out.Script.String() != common.NewThresholdScript(1).String() {
			return fmt.Errorf("invalid mint output script %s", out.Script.String())
		}
		if len(out.Keys) != 1 {
			return fmt.Errorf("invalid mint output keys %d", len(out.Keys))
		}
		n := nodes[i]
		in := fmt.Sprintf("MINTKERNELNODE%d", mint.Batch)
		seed := crypto.NewHash([]byte(n.Signer.String() + in))
		r := crypto.NewKeyFromSeed(append(seed[:], seed[:]...))
		if r.Public() != out.Mask {
			return fmt.Errorf("invalid mint output mask %s %s", r.Public().String(), out.Mask.String())
		}
		ghost := crypto.ViewGhostOutputKey(&out.Keys[0], &n.Payee.PrivateViewKey, &out.Mask, uint64(i))
		if *ghost != n.Payee.PublicSpendKey {
			return fmt.Errorf("invalid mint output signature %s %s", n.Payee.PublicSpendKey.String(), ghost.String())
		}
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
	if hours%24 < config.KernelMintTimeBegin || hours%24 > config.KernelMintTimeEnd {
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
		logger.Println("ReadLastMintDistribution ERROR", err)
		return 0, common.Zero
	}
	logger.Println("checkMintPossibility OLD", pool, total, light, full, batch, dist.Amount, dist.Batch)

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
	logger.Println("checkMintPossibility NEW", pool, total, light, full, amount, batch, dist.Amount, dist.Batch)
	return batch, amount
}

func (node *Node) sortMintNodes() []*common.Node {
	var nodes []*common.Node
	for _, n := range node.ConsensusNodes {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		a := nodes[i].IdForNetwork(node.networkId)
		b := nodes[j].IdForNetwork(node.networkId)
		return a.String() < b.String()
	})
	return nodes
}

func (node *Node) SortAllNodesByTimestampAndId() []*common.Node {
	nodes := node.persistStore.ReadAllNodes()
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Timestamp < nodes[j].Timestamp {
			return true
		}
		if nodes[i].Timestamp > nodes[j].Timestamp {
			return false
		}
		a := nodes[i].IdForNetwork(node.networkId)
		b := nodes[j].IdForNetwork(node.networkId)
		return a.String() < b.String()
	})
	return nodes
}
