package kernel

import (
	"fmt"
	"time"

	"github.com/MixinNetwork/mixin/common"
	"github.com/MixinNetwork/mixin/config"
	"github.com/MixinNetwork/mixin/logger"
)

func (chain *Chain) AggregateRoundSpace() {
	logger.Printf("AggregateRoundSpace(%s)\n", chain.ChainId)
	defer close(chain.slc)

	batch, round, err := chain.persistStore.ReadRoundSpaceCheckpoint(chain.ChainId)
	if err != nil {
		panic(err)
	}
	logger.Printf("AggregateRoundSpace(%s) begin with %d:%d\n", chain.ChainId, batch, round)

	wait := time.Duration(chain.node.custom.Node.KernelOprationPeriod/2) * time.Second
	for chain.running {
		if cs := chain.State; cs == nil {
			logger.Printf("AggregateRoundSpace(%s) no state yet\n", chain.ChainId)
			chain.waitOrDone(wait)
			continue
		}
		frn := chain.State.FinalRound.Number
		if frn < round {
			panic(fmt.Errorf("AggregateRoundSpace(%s) waiting %d %d", chain.ChainId, frn, round))
		}
		if frn < round+1 {
			chain.waitOrDone(wait)
			continue
		}

		nextTime, err := chain.readFinalRoundTimestamp(round + 1)
		if err != nil {
			logger.Verbosef("AggregateRoundSpace(%s) ERROR readFinalRoundTimestamp %d %v\n", chain.ChainId, round+1, err)
			continue
		}
		checkTime, err := chain.readFinalRoundTimestamp(round)
		if err != nil {
			logger.Verbosef("AggregateRoundSpace(%s) ERROR readFinalRoundTimestamp %d %v\n", chain.ChainId, round, err)
			continue
		}

		since := checkTime - chain.node.Epoch
		batch := uint64(since/3600000000000) / 24
		if batch > chain.node.LastMint+1 {
			logger.Verbosef("AggregateRoundSpace(%s) ERROR batch future %d %d %s", batch, chain.node.LastMint, time.Unix(0, int64(checkTime)))
			chain.waitOrDone(wait)
			continue
		}
		space := &common.RoundSpace{
			NodeId:   chain.ChainId,
			Batch:    batch,
			Round:    round,
			Duration: nextTime - checkTime,
		}
		if space.Round == 0 {
			space.Duration = 0
		}

		if space.Duration > uint64(config.CheckpointDuration) {
			logger.Printf("AggregateRoundSpace(%s) => large gap %d:%d %d", chain.ChainId, batch, round, space.Duration)
		} else {
			space.Duration = 0
		}

		err = chain.persistStore.WriteRoundSpaceAndState(space)
		if err != nil {
			logger.Verbosef("AggregateRoundSpace(%s) ERROR WriteRoundSpaceAndState %d %v\n", chain.ChainId, round, err)
			continue
		}
		round = round + 1
	}

	batch, round, err = chain.persistStore.ReadRoundSpaceCheckpoint(chain.ChainId)
	if err != nil {
		panic(err)
	}
	logger.Printf("AggregateRoundSpace(%s) end with %d:%d\n", chain.ChainId, batch, round)
}

func (chain *Chain) readFinalRoundTimestamp(round uint64) (uint64, error) {
	snapshots, err := chain.persistStore.ReadSnapshotsForNodeRound(chain.ChainId, round)
	if err != nil {
		return 0, err
	}
	if len(snapshots) == 0 {
		panic(fmt.Errorf("readFinalRoundTimestamp(%s, %d) empty round", chain.ChainId, round))
	}

	rawSnapshots := make([]*common.Snapshot, len(snapshots))
	for i, s := range snapshots {
		rawSnapshots[i] = s.Snapshot
	}
	start, _, hash := ComputeRoundHash(chain.ChainId, round, rawSnapshots)

	r, err := chain.persistStore.ReadRound(hash)
	if err != nil {
		return 0, err
	}
	if r.Timestamp != start || r.Number != round {
		panic(fmt.Errorf("readFinalRoundTimestamp(%s, %d) => malformed round attributes %d %d", chain.ChainId, round, start, r.Timestamp))
	}
	return r.Timestamp, nil
}
