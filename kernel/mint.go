package kernel

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/crypto"
	"github.com/MixinNetwork/mixin/kernel/internal/clock"
	"github.com/MixinNetwork/mixin/logger"
	"github.com/dgraph-io/badger/v2"
)

const (
	MainnetMintPeriodForkBatch           = 72
	MainnetMintPeriodForkTimeBegin       = 6
	MainnetMintPeriodForkTimeEnd         = 18
	MainnetMintWorkDistributionForkBatch = 729
	MainnetMintTransactionV2ForkBatch    = 739
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

func (chain *Chain) AggregateMintWork() {
	logger.Printf("AggregateMintWork(%s)\n", chain.ChainId)
	defer close(chain.wlc)

	round, err := chain.persistStore.ReadWorkOffset(chain.ChainId)
	if err != nil {
		panic(err)
	}
	logger.Printf("AggregateMintWork(%s) begin with %d\n", chain.ChainId, round)

	period := time.Duration(chain.node.custom.Node.KernelOprationPeriod) * time.Second
	fork := uint64(SnapshotRoundDayLeapForkHack.UnixNano())
	for chain.running {
		if cs := chain.State; cs == nil {
			logger.Printf("AggregateMintWork(%s) no state yet\n", chain.ChainId)
			time.Sleep(period)
			continue
		}
		crn := chain.State.CacheRound.Number
		if crn < round {
			panic(fmt.Errorf("AggregateMintWork(%s) waiting %d %d", chain.ChainId, crn, round))
		}
		snapshots, err := chain.persistStore.ReadSnapshotWorksForNodeRound(chain.ChainId, round)
		if err != nil {
			logger.Verbosef("AggregateMintWork(%s) ERROR ReadSnapshotsForNodeRound %s\n", chain.ChainId, err.Error())
			continue
		}
		if len(snapshots) == 0 {
			time.Sleep(period)
			continue
		}
		for chain.running {
			if chain.node.networkId.String() == config.MainnetId && snapshots[0].Timestamp < fork {
				snapshots = nil
			}
			err = chain.persistStore.WriteRoundWork(chain.ChainId, round, snapshots)
			if err == nil {
				break
			}
			if errors.Is(err, badger.ErrConflict) {
				logger.Verbosef("AggregateMintWork(%s) ERROR WriteRoundWork %s\n", chain.ChainId, err.Error())
				time.Sleep(100 * time.Millisecond)
				continue
			}
			panic(err)
		}
		if round < crn {
			round = round + 1
		} else {
			time.Sleep(period)
		}
	}

	logger.Printf("AggregateMintWork(%s) end with %d\n", chain.ChainId, round)
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
			logger.Println(node.IdForNetwork, "tryToMintKernelNode", err)
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

func (node *Node) PledgeAmount(ts uint64) common.Integer {
	if ts < node.Epoch {
		return pledgeAmount(0)
	}
	since := uint64(ts) - node.Epoch
	return pledgeAmount(time.Duration(since))
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

	if node.networkId.String() == config.MainnetId && batch < MainnetMintTransactionV2ForkBatch {
		return node.buildMintTransactionV1(timestamp, validateOnly)
	}

	accepted := node.NodesListWithoutState(timestamp, true)
	mints, err := node.distributeMintByWorks(accepted, amount, timestamp)
	if err != nil {
		logger.Printf("buildMintTransaction ERROR %s\n", err.Error())
		return nil
	}

	tx := common.NewTransaction(common.XINAssetId)
	tx.AddKernelNodeMintInput(uint64(batch), amount)
	script := common.NewThresholdScript(1)
	total := common.NewInteger(0)
	for _, m := range mints {
		in := fmt.Sprintf("MINTKERNELNODE%d", batch)
		si := crypto.NewHash([]byte(m.Signer.String() + in))
		seed := append(si[:], si[:]...)
		tx.AddScriptOutput([]*common.Address{&m.Payee}, script, m.Work, seed)
		total = total.Add(m.Work)
	}
	if total.Cmp(amount) > 0 {
		panic(fmt.Errorf("buildMintTransaction %s %s", amount, total))
	}

	if diff := amount.Sub(total); diff.Sign() > 0 {
		addr := common.NewAddressFromSeed(make([]byte, 64))
		script := common.NewThresholdScript(common.Operator64)
		in := fmt.Sprintf("MINTKERNELNODE%dDIFF", batch)
		si := crypto.NewHash([]byte(addr.String() + in))
		seed := append(si[:], si[:]...)
		tx.AddScriptOutput([]*common.Address{&addr}, script, diff, seed)
	}
	return tx.AsLatestVersion()
}

func (node *Node) tryToMintKernelNode() error {
	signed := node.buildMintTransaction(node.GraphTimestamp, false)
	if signed == nil {
		return nil
	}

	if signed.Version == 1 {
		err := signed.SignInputV1(node.persistStore, 0, []*common.Address{&node.Signer})
		if err != nil {
			return err
		}
	} else {
		err := signed.SignInput(node.persistStore, 0, []*common.Address{&node.Signer})
		if err != nil {
			return err
		}
	}
	err := signed.Validate(node.persistStore)
	if err != nil {
		return err
	}
	err = node.persistStore.CachePutTransaction(signed)
	if err != nil {
		return err
	}
	return node.chain.AppendSelfEmpty(&common.Snapshot{
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
		th := hex.EncodeToString(tx.PayloadMarshal())
		sh := hex.EncodeToString(signed.PayloadMarshal())
		return fmt.Errorf("malformed mint transaction at %d %s %s", timestamp, th, sh)
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

type CNodeWork struct {
	CNode
	Work common.Integer
}

func (node *Node) ListMintWorks(batch uint64) (map[crypto.Hash][2]uint64, error) {
	now := node.Epoch + batch*uint64(time.Hour*24)
	list := node.NodesListWithoutState(now, true)
	cids := make([]crypto.Hash, len(list))
	for i, n := range list {
		cids[i] = n.IdForNetwork
	}
	day := now / (uint64(time.Hour) * 24)
	works, err := node.persistStore.ListNodeWorks(cids, uint32(day))
	return works, err
}

// a = average work
// for x > 7a, y = 2a
// for 7a > x > a, y = 1/6x + 5/6a
// for a > x > 1/7a, y = x
// for x < 1/7a, y = 1/7a
func (node *Node) distributeMintByWorks(accepted []*CNode, base common.Integer, timestamp uint64) ([]*CNodeWork, error) {
	mints := make([]*CNodeWork, len(accepted))
	cids := make([]crypto.Hash, len(accepted))
	for i, n := range accepted {
		cids[i] = n.IdForNetwork
		mints[i] = &CNodeWork{CNode: *n}
	}
	epoch := node.Epoch / (uint64(time.Hour) * 24)
	day := timestamp / (uint64(time.Hour) * 24)
	if day < epoch {
		panic(fmt.Errorf("invalid mint day %d %d", epoch, day))
	}
	if day-epoch == 0 {
		work := base.Div(len(mints))
		for _, m := range mints {
			m.Work = work
		}
		return mints, nil
	}

	works, err := node.persistStore.ListNodeWorks(cids, uint32(day))
	if err != nil {
		return nil, err
	}
	thr, agg := int(node.ConsensusThreshold(timestamp)), 0
	for _, w := range works {
		if w[0] > 0 {
			agg += 1
		}
	}
	if agg < thr {
		return nil, fmt.Errorf("distributeMintByWorks not ready yet %d %d %d %d", day, len(mints), agg, thr)
	}

	works, err = node.persistStore.ListNodeWorks(cids, uint32(day)-1)
	if err != nil {
		return nil, err
	}

	var valid int
	var min, max, total common.Integer
	for _, m := range mints {
		w := works[m.IdForNetwork]
		m.Work = common.NewInteger(w[0]).Mul(120).Div(100)
		sign := common.NewInteger(w[1])
		if sign.Sign() > 0 {
			m.Work = m.Work.Add(sign)
		}
		if m.Work.Sign() == 0 {
			continue
		}
		valid += 1
		if min.Sign() == 0 {
			min = m.Work
		} else if m.Work.Cmp(min) < 0 {
			min = m.Work
		}
		if m.Work.Cmp(max) > 0 {
			max = m.Work
		}
		total = total.Add(m.Work)
	}
	if valid < thr {
		return nil, fmt.Errorf("distributeMintByWorks not valid %d %d %d %d", day, len(mints), thr, valid)
	}

	total = total.Sub(min).Sub(max)
	avg := total.Div(valid - 2)
	if avg.Sign() == 0 {
		return nil, fmt.Errorf("distributeMintByWorks not valid %d %d %d %d", day, len(mints), thr, valid)
	}

	total = common.NewInteger(0)
	upper, lower := avg.Mul(7), avg.Div(7)
	for _, m := range mints {
		if m.Work.Cmp(upper) >= 0 {
			m.Work = avg.Mul(2)
		} else if m.Work.Cmp(avg) >= 0 {
			m.Work = m.Work.Div(6).Add(avg.Mul(5).Div(6))
		} else if m.Work.Cmp(lower) <= 0 {
			m.Work = avg.Div(7)
		}
		total = total.Add(m.Work)
	}

	for _, m := range mints {
		rat := m.Work.Ration(total)
		m.Work = rat.Product(base)
	}
	return mints, nil
}
