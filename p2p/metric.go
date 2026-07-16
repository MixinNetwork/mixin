package p2p

import (
	"encoding/json"
	"sync/atomic"
)

type MetricPool struct {
	enabled atomic.Bool

	PeerMessageTypePing               uint32 `json:"ping"`
	PeerMessageTypeAuthentication     uint32 `json:"authentication"`
	PeerMessageTypeGraph              uint32 `json:"graph"`
	PeerMessageTypeSnapshotConfirm    uint32 `json:"snapshot-confirm"`
	PeerMessageTypeTransactionRequest uint32 `json:"transaction-request"`
	PeerMessageTypeTransaction        uint32 `json:"transaction"`

	PeerMessageTypeSnapshotAnnouncement       uint32 `json:"snapshot-announcement"`
	PeerMessageTypeSnapshotCommitment         uint32 `json:"snapshot-commitment"`
	PeerMessageTypeTransactionChallenge       uint32 `json:"transaciton-challenge"`
	PeerMessageTypeSnapshotResponse           uint32 `json:"snapshot-response"`
	PeerMessageTypeSnapshotFinalization       uint32 `json:"snapshot-finalization"`
	PeerMessageTypePreCommitments             uint32 `json:"commitments"`
	PeerMessageTypeFullChallenge              uint32 `json:"full-challenge"`
	PeerMessageTypeTransactionBundle          uint32 `json:"transaction-bundle"`
	PeerMessageTypeFinalizedTransactionBundle uint32 `json:"finalized-transaction-bundle"`

	PeerMessageTypeRelay uint32 `json:"relay"`
}

type metricPoolJSON MetricPool

func (mp *MetricPool) handle(msg uint8) {
	if !mp.enabled.Load() {
		return
	}

	switch msg {
	case PeerMessageTypePing:
		atomic.AddUint32(&mp.PeerMessageTypePing, 1)
	case PeerMessageTypeAuthentication:
		atomic.AddUint32(&mp.PeerMessageTypeAuthentication, 1)
	case PeerMessageTypeGraph:
		atomic.AddUint32(&mp.PeerMessageTypeGraph, 1)
	case PeerMessageTypeSnapshotConfirm:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotConfirm, 1)
	case PeerMessageTypeTransactionRequest:
		atomic.AddUint32(&mp.PeerMessageTypeTransactionRequest, 1)
	case PeerMessageTypeTransaction:
		atomic.AddUint32(&mp.PeerMessageTypeTransaction, 1)
	case PeerMessageTypePreCommitments:
		atomic.AddUint32(&mp.PeerMessageTypePreCommitments, 1)
	case PeerMessageTypeTransactionBundle:
		atomic.AddUint32(&mp.PeerMessageTypeTransactionBundle, 1)
	case PeerMessageTypeFinalizedTransactionBundle:
		atomic.AddUint32(&mp.PeerMessageTypeFinalizedTransactionBundle, 1)
	case PeerMessageTypeBatchSnapshotAnnouncement:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotAnnouncement, 1)
	case PeerMessageTypeBatchSnapshotCommitment:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotCommitment, 1)
	case PeerMessageTypeBatchTransactionChallenge:
		atomic.AddUint32(&mp.PeerMessageTypeTransactionChallenge, 1)
	case PeerMessageTypeBatchSnapshotResponse:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotResponse, 1)
	case PeerMessageTypeBatchFullChallenge:
		atomic.AddUint32(&mp.PeerMessageTypeFullChallenge, 1)
	case PeerMessageTypeBatchSnapshotFinalization:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotFinalization, 1)
	case PeerMessageTypeRelay:
		atomic.AddUint32(&mp.PeerMessageTypeRelay, 1)
	}
}

func (mp *MetricPool) String() string {
	b, err := mp.MarshalJSON()
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (mp *MetricPool) MarshalJSON() ([]byte, error) {
	snapshot := MetricPool{
		PeerMessageTypePing:                       atomic.LoadUint32(&mp.PeerMessageTypePing),
		PeerMessageTypeAuthentication:             atomic.LoadUint32(&mp.PeerMessageTypeAuthentication),
		PeerMessageTypeGraph:                      atomic.LoadUint32(&mp.PeerMessageTypeGraph),
		PeerMessageTypeSnapshotConfirm:            atomic.LoadUint32(&mp.PeerMessageTypeSnapshotConfirm),
		PeerMessageTypeTransactionRequest:         atomic.LoadUint32(&mp.PeerMessageTypeTransactionRequest),
		PeerMessageTypeTransaction:                atomic.LoadUint32(&mp.PeerMessageTypeTransaction),
		PeerMessageTypeSnapshotAnnouncement:       atomic.LoadUint32(&mp.PeerMessageTypeSnapshotAnnouncement),
		PeerMessageTypeSnapshotCommitment:         atomic.LoadUint32(&mp.PeerMessageTypeSnapshotCommitment),
		PeerMessageTypeTransactionChallenge:       atomic.LoadUint32(&mp.PeerMessageTypeTransactionChallenge),
		PeerMessageTypeSnapshotResponse:           atomic.LoadUint32(&mp.PeerMessageTypeSnapshotResponse),
		PeerMessageTypeSnapshotFinalization:       atomic.LoadUint32(&mp.PeerMessageTypeSnapshotFinalization),
		PeerMessageTypePreCommitments:             atomic.LoadUint32(&mp.PeerMessageTypePreCommitments),
		PeerMessageTypeFullChallenge:              atomic.LoadUint32(&mp.PeerMessageTypeFullChallenge),
		PeerMessageTypeTransactionBundle:          atomic.LoadUint32(&mp.PeerMessageTypeTransactionBundle),
		PeerMessageTypeFinalizedTransactionBundle: atomic.LoadUint32(&mp.PeerMessageTypeFinalizedTransactionBundle),
		PeerMessageTypeRelay:                      atomic.LoadUint32(&mp.PeerMessageTypeRelay),
	}
	return json.Marshal((*metricPoolJSON)(&snapshot))
}
