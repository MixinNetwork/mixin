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
	"github.com/dgraph-io/badger/v4"
)

var (
	MintPool        = common.NewInteger(500000)
	MintLiquidity   = common.NewInteger(500000)
	MintYearPercent = common.NewInteger(10).Ration(common.NewInteger(100))
)

const (
	MintYearDays              = 365
	KernelNetworkLegacyEnding = 1706
	OneDay                    = 24 * uint64(time.Hour)
)

func (chain *Chain) AggregateMintWork() {
	logger.Printf("AggregateMintWork(%s)\n", chain.ChainId)
	defer close(chain.wlc)

	round, err := chain.persistStore.ReadWorkOffset(chain.ChainId)
	if err != nil {
		panic(err)
	}
	logger.Printf("AggregateMintWork(%s) begin with %d\n", chain.ChainId, round)

	wait := time.Duration(chain.node.custom.Node.KernelOprationPeriod/2) * time.Second

	for chain.running {
		if cs := chain.State; cs == nil {
			logger.Printf("AggregateMintWork(%s) no state yet\n", chain.ChainId)
			chain.waitOrDone(wait)
			continue
		}
		// FIXME Here continues to update the cache round mostly because no way to
		// decide the last round of a removed node. The fix is to penalize the late
		// spending of a node remove output, i.e. the node remove output must be
		// used as soon as possible.
		// A better fix is to init some transaction that references the node removal
		// all automatically from kernel.
		// Another fix is to utilize the light node to reference the node removal
		// and incentivize the first light nodes that do this.
		// we don't care the round state final or cache, it must has subsequent snapshots
		md, ok := chain.checkRoundMature(round)
		if !ok {
			chain.waitOrDone(wait)
			continue
		}
		snapshots, err := chain.persistStore.ReadSnapshotWorksForNodeRound(chain.ChainId, round)
		if err != nil {
			logger.Printf("AggregateMintWork(%s) ERROR ReadSnapshotsForNodeRound %s\n", chain.ChainId, err.Error())
			continue
		}
		rd := snapshots[0].Timestamp / OneDay
		if rd > md {
			panic(fmt.Errorf("AggregateMintWork(%s) %d %d %d", chain.ChainId, round, rd, md))
		}
		err = chain.writeRoundWork(round, snapshots, rd == md)
		if err != nil {
			panic(err)
		}
		if round < chain.State.CacheRound.Number {
			round = round + 1
		} else {
			chain.waitOrDone(wait)
		}
	}

	logger.Printf("AggregateMintWork(%s) end with %d\n", chain.ChainId, round)
}

func (chain *Chain) checkRoundMature(round uint64) (uint64, bool) {
	cache := chain.State.CacheRound
	if cache.Number < round {
		panic(fmt.Errorf("AggregateMintWork(%s) waiting %d %d", chain.ChainId, cache.Number, round))
	}
	if cache.Number == round {
		return 0, false
	}
	if cache.Number == round+1 {
		if len(cache.Snapshots) < 1 {
			return 0, false
		}
		return cache.Snapshots[0].Timestamp / OneDay, true
	}
	snapshots, err := chain.persistStore.ReadSnapshotWorksForNodeRound(chain.ChainId, round+1)
	if err != nil {
		panic(err)
	}
	return snapshots[0].Timestamp / OneDay, true
}

func (chain *Chain) writeRoundWork(round uint64, works []*common.SnapshotWork, credit bool) error {
	credit = credit || (chain.node.networkId.String() == config.KernelNetworkId &&
		(works[0].Timestamp-chain.node.Epoch)/OneDay < mainnetMintDayGapSkipForkBatch)
	for chain.running {
		err := chain.persistStore.WriteRoundWork(chain.ChainId, round, works, credit)
		if err == nil {
			return nil
		}
		if errors.Is(err, badger.ErrConflict) {
			logger.Verbosef("AggregateMintWork(%s) ERROR WriteRoundWork %s\n", chain.ChainId, err.Error())
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return err
	}
	return fmt.Errorf("chain %s is done", chain.ChainId)
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
			cur, err := node.persistStore.ReadCustodian(node.GraphTimestamp)
			if err != nil {
				panic(err)
			}
			err = node.tryToMintUniversal(cur)
			logger.Println(node.IdForNetwork, "tryToMintKernelUniversal", err)
		}
	}
}

func (node *Node) tryToMintUniversal(custodianRequest *common.CustodianUpdateRequest) error {
	eid := node.electSnapshotNode(common.TransactionTypeMint, node.GraphTimestamp)
	if eid != node.IdForNetwork {
		return fmt.Errorf("universal mint operation at %d only by %s not me", node.GraphTimestamp, eid)
	}
	signed := node.buildUniversalMintTransaction(custodianRequest, node.GraphTimestamp, false)
	if signed == nil {
		return nil
	}

	err := signed.SignInput(node.persistStore, 0, []*common.Address{&node.Signer})
	if err != nil {
		return err
	}
	err = signed.Validate(node.persistStore, node.GraphTimestamp, false)
	if err != nil {
		return err
	}
	err = node.persistStore.CachePutTransaction(signed)
	if err != nil {
		return err
	}
	s := &common.Snapshot{
		Version: common.SnapshotVersionCommonEncoding,
		NodeId:  node.IdForNetwork,
	}
	s.AddSoleTransaction(signed.PayloadHash())
	logger.Println("tryToMintUniversal", signed.PayloadHash(), hex.EncodeToString(signed.Marshal()))
	return node.chain.AppendSelfEmpty(s)
}

func (node *Node) buildUniversalMintTransaction(custodianRequest *common.CustodianUpdateRequest, timestamp uint64, validateOnly bool) *common.VersionedTransaction {
	batch, amount := node.checkUniversalMintPossibility(timestamp, validateOnly)
	if amount.Sign() <= 0 || batch <= KernelNetworkLegacyEnding {
		return nil
	}

	kernel := amount.Div(10).Mul(5)
	accepted := node.NodesListWithoutState(timestamp, true)
	mints, err := node.distributeKernelMintByWorks(accepted, kernel, timestamp)
	if err != nil {
		logger.Printf("buildUniversalMintTransaction ERROR %s\n", err.Error())
		return nil
	}

	consensusSnap, referencedBy, err := node.persistStore.ReadLastConsensusSnapshot()
	if err != nil {
		logger.Printf("buildUniversalMintTransaction ERROR %s\n", err.Error())
		return nil
	}
	if referencedBy != nil {
		logger.Printf("buildUniversalMintTransaction consensus reference %s\n", referencedBy)
		return nil
	}
	tx := node.NewTransaction(common.XINAssetId)
	tx.AddUniversalMintInput(uint64(batch), amount)
	tx.References = []crypto.Hash{consensusSnap.SoleTransaction()}

	total := common.NewInteger(0)
	for _, m := range mints {
		in := fmt.Sprintf("MINTKERNELNODE%d", batch)
		si := crypto.Blake3Hash([]byte(m.Signer.String() + in))
		seed := append(si[:], si[:]...)
		script := common.NewThresholdScript(1)
		tx.AddScriptOutput([]*common.Address{&m.Payee}, script, m.Work, seed)
		total = total.Add(m.Work)
	}
	if total.Cmp(amount) > 0 {
		panic(fmt.Errorf("buildUniversalMintTransaction %s %s", amount, total))
	}

	safe := amount.Div(10).Mul(4)
	custodian := custodianRequest.Custodian
	in := fmt.Sprintf("MINTCUSTODIANACCOUNT%d", batch)
	si := crypto.Blake3Hash([]byte(custodian.String() + in))
	seed := append(si[:], si[:]...)
	script := common.NewThresholdScript(1)
	tx.AddScriptOutput([]*common.Address{custodian}, script, safe, seed)
	total = total.Add(safe)
	if total.Cmp(amount) > 0 {
		panic(fmt.Errorf("buildUniversalMintTransaction %s %s", amount, total))
	}

	// TODO use real light mint account when light node online
	light := amount.Sub(total)
	addr := common.NewAddressFromSeed(make([]byte, 64))
	script = common.NewThresholdScript(common.Operator64)
	in = fmt.Sprintf("MINTLIGHTACCOUNT%d", batch)
	si = crypto.Blake3Hash([]byte(addr.String() + in))
	seed = append(si[:], si[:]...)
	tx.AddScriptOutput([]*common.Address{&addr}, script, light, seed)
	return tx.AsVersioned()
}

func (node *Node) PoolSize() (common.Integer, error) {
	dist := node.lastMintDistribution()
	return poolSizeUniversal(int(dist.Batch)), nil
}

// this is the new mixin kernel, with 1706 batch, e.g. 2023/10/31 as
// the last mint batch for the legacy kernel, and the first mint
// for this kernel will be 1707
func (node *Node) lastMintDistribution() *common.MintData {
	dist, err := node.persistStore.ReadLastMintDistribution(^uint64(0))
	if err != nil {
		panic(err)
	}
	if dist == nil {
		return &common.MintData{
			Batch:  KernelNetworkLegacyEnding,
			Amount: common.NewIntegerFromString("89.87671232"),
		}
	}
	if dist.Batch < KernelNetworkLegacyEnding {
		panic(dist.Batch)
	}
	return &dist.MintData
}

func poolSizeUniversal(batch int) common.Integer {
	mint, pool := common.Zero, MintPool
	for i := 0; i < batch/MintYearDays; i++ {
		year := MintYearPercent.Product(pool)
		mint = mint.Add(year)
		pool = pool.Sub(year)
	}
	year := MintYearPercent.Product(pool)
	day := year.Div(MintYearDays)
	if count := batch % MintYearDays; count > 0 {
		mint = mint.Add(day.Mul(count))
	}
	if mint.Sign() > 0 {
		return MintPool.Sub(mint)
	}
	return MintPool
}

func mintBatchSize(batch uint64) common.Integer {
	pool, years := MintPool, batch/MintYearDays
	if years > 10000 {
		panic(years)
	}
	for i := 0; i < int(years); i++ {
		year := MintYearPercent.Product(pool)
		pool = pool.Sub(year)
	}
	year := MintYearPercent.Product(pool)
	return year.Div(MintYearDays)
}

func mintMultiBatchesSize(old, batch uint64) common.Integer {
	if old >= batch {
		panic(batch)
	}
	var amount common.Integer
	for i := old + 1; i <= batch; i++ {
		amount = amount.Add(mintBatchSize(i))
	}
	return amount
}

func (node *Node) validateMintSnapshot(snap *common.Snapshot, tx *common.VersionedTransaction) error {
	timestamp := snap.Timestamp
	if snap.Timestamp == 0 && snap.NodeId == node.IdForNetwork {
		timestamp = uint64(clock.Now().UnixNano())
	}
	eid := node.electSnapshotNode(common.TransactionTypeMint, timestamp)
	if eid != snap.NodeId {
		return fmt.Errorf("univernal mint operation at %d only by %s not %s", timestamp, eid, snap.NodeId)
	}

	var signed *common.VersionedTransaction
	cur, err := node.persistStore.ReadCustodian(timestamp)
	if err != nil {
		return err
	}
	signed = node.buildUniversalMintTransaction(cur, timestamp, true)
	if signed == nil {
		return fmt.Errorf("no universal mint available at %d", timestamp)
	}

	if tx.PayloadHash() != signed.PayloadHash() {
		th := hex.EncodeToString(tx.PayloadMarshal())
		sh := hex.EncodeToString(signed.PayloadMarshal())
		return fmt.Errorf("malformed mint transaction at %d %s %s", timestamp, th, sh)
	}
	return nil
}

func (node *Node) checkUniversalMintPossibility(timestamp uint64, validateOnly bool) (uint64, common.Integer) {
	if timestamp <= node.Epoch {
		return 0, common.Zero
	}

	since := timestamp - node.Epoch
	hours := int(since / 3600000000000)
	batch := uint64(hours / 24)
	if batch < 1 {
		return 0, common.Zero
	}
	kmb, kme := config.KernelMintTimeBegin, config.KernelMintTimeEnd
	if hours%24 < kmb || hours%24 > kme {
		return 0, common.Zero
	}

	dist := node.lastMintDistribution()
	logger.Verbosef("checkUniversalMintPossibility OLD %d %s %d\n",
		batch, dist.Amount, dist.Batch)

	if batch < dist.Batch {
		return 0, common.Zero
	}
	if batch == dist.Batch {
		if validateOnly {
			return batch, dist.Amount
		}
		return 0, common.Zero
	}

	amount := mintMultiBatchesSize(dist.Batch, batch)
	logger.Verbosef("checkUniversalMintPossibility NEW %s %d %s %d\n",
		amount, batch, dist.Amount, dist.Batch)
	return batch, amount
}

type CNodeWork struct {
	CNode
	Work common.Integer
}

func (node *Node) ListMintWorks(batch uint64) (map[crypto.Hash][2]uint64, error) {
	now := node.Epoch + batch*OneDay
	list := node.NodesListWithoutState(now, true)
	cids := make([]crypto.Hash, len(list))
	for i, n := range list {
		cids[i] = n.IdForNetwork
	}
	return node.persistStore.ListNodeWorks(cids, uint32(now/OneDay))
}

func (node *Node) ListRoundSpaces(cids []crypto.Hash, day uint64) (map[crypto.Hash][]*common.RoundSpace, error) {
	epoch := node.Epoch / OneDay
	spaces := make(map[crypto.Hash][]*common.RoundSpace)
	for _, id := range cids {
		ns, err := node.persistStore.ReadNodeRoundSpacesForBatch(id, day-epoch)
		if err != nil {
			return nil, err
		}
		spaces[id] = ns
	}
	return spaces, nil
}

// a = average work
// for x > 7a, y = 2a
// for 7a > x > a, y = 1/6x + 5/6a
// for a > x > 1/7a, y = x
// for x < 1/7a, y = 1/7a
func (node *Node) distributeKernelMintByWorks(accepted []*CNode, base common.Integer, timestamp uint64) ([]*CNodeWork, error) {
	mints := make([]*CNodeWork, len(accepted))
	cids := make([]crypto.Hash, len(accepted))
	for i, n := range accepted {
		cids[i] = n.IdForNetwork
		mints[i] = &CNodeWork{CNode: *n}
	}
	epoch := node.Epoch / OneDay
	day := timestamp / OneDay
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

	thr := int(node.ConsensusThreshold(timestamp, false))
	err := node.validateWorksAndSpacesAggregator(cids, thr, day)
	if err != nil {
		return nil, fmt.Errorf("distributeKernelMintByWorks not ready yet %d %v", day, err)
	}

	works, err := node.persistStore.ListNodeWorks(cids, uint32(day)-1)
	if err != nil {
		return nil, err
	}
	spaces, err := node.ListRoundSpaces(cids, day-1)
	if err != nil {
		return nil, err
	}

	var valid int
	var minW, maxW, totalW common.Integer
	for _, m := range mints {
		ns := spaces[m.IdForNetwork]
		if len(ns) > 0 {
			// TODO enable this for universal mint distributions, need to ensure all nodes
			// have their own transaction monitor, send some regular transactions
			// otherwise this will not work in low transaction conditions
			logger.Verbosef("node spaces %s %d %d\n", m.IdForNetwork, ns[0].Batch, len(ns))
		}

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
		if minW.Sign() == 0 {
			minW = m.Work
		} else if m.Work.Cmp(minW) < 0 {
			minW = m.Work
		}
		if m.Work.Cmp(maxW) > 0 {
			maxW = m.Work
		}
		totalW = totalW.Add(m.Work)
	}
	if valid < thr {
		return nil, fmt.Errorf("distributeKernelMintByWorks not valid %d %d %d %d",
			day, len(mints), thr, valid)
	}

	totalW = totalW.Sub(minW).Sub(maxW)
	avg := totalW.Div(valid - 2)
	if avg.Sign() == 0 {
		return nil, fmt.Errorf("distributeKernelMintByWorks not valid %d %d %d %d",
			day, len(mints), thr, valid)
	}

	totalW = common.NewInteger(0)
	upper, lower := avg.Mul(7), avg.Div(7)
	for _, m := range mints {
		if m.Work.Cmp(upper) >= 0 {
			m.Work = avg.Mul(2)
		} else if m.Work.Cmp(avg) >= 0 {
			m.Work = m.Work.Div(6).Add(avg.Mul(5).Div(6))
		} else if m.Work.Cmp(lower) <= 0 {
			m.Work = avg.Div(7)
		}
		totalW = totalW.Add(m.Work)
	}

	for _, m := range mints {
		rat := m.Work.Ration(totalW)
		m.Work = rat.Product(base)
	}
	return mints, nil
}

func (node *Node) validateWorksAndSpacesAggregator(cids []crypto.Hash, thr int, day uint64) error {
	worksAgg, spacesAgg := 0, 0

	works, err := node.persistStore.ListNodeWorks(cids, uint32(day))
	if err != nil {
		return err
	}
	for _, w := range works {
		if w[0] > 0 {
			worksAgg += 1
		}
	}
	if worksAgg < thr {
		return fmt.Errorf("validateWorksAndSpacesAggregator works not ready yet %d %d %d %d",
			day, len(works), worksAgg, thr)
	}

	spaces, err := node.persistStore.ListAggregatedRoundSpaceCheckpoints(cids)
	if err != nil {
		return err
	}
	epoch := node.Epoch / OneDay
	batch := day - epoch
	for _, s := range spaces {
		if s.Batch >= batch {
			spacesAgg += 1
		}
	}
	if spacesAgg < thr || worksAgg != spacesAgg {
		return fmt.Errorf("validateWorksAndSpacesAggregator spaces not ready yet %d %d %d %d %d",
			batch, len(spaces), spacesAgg, worksAgg, thr)
	}

	return nil
}
