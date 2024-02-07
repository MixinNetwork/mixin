package p2p

import (
	"encoding/json"
	"sync/atomic"
)

type MetricPool struct {
	enabled bool

	PeerMessageTypePing               uint32 `json:"ping"`
	PeerMessageTypeAuthentication     uint32 `json:"authentication"`
	PeerMessageTypeGraph              uint32 `json:"graph"`
	PeerMessageTypeSnapshotConfirm    uint32 `json:"snapshot-confirm"`
	PeerMessageTypeTransactionRequest uint32 `json:"transaction-request"`
	PeerMessageTypeTransaction        uint32 `json:"transaction"`

	PeerMessageTypeSnapshotAnnouncement uint32 `json:"snapshot-announcement"`
	PeerMessageTypeSnapshotCommitment   uint32 `json:"snapshot-commitment"`
	PeerMessageTypeTransactionChallenge uint32 `json:"transaciton-challenge"`
	PeerMessageTypeSnapshotResponse     uint32 `json:"snapshot-response"`
	PeerMessageTypeSnapshotFinalization uint32 `json:"snapshot-finalization"`
	PeerMessageTypeCommitments          uint32 `json:"commitments"`
	PeerMessageTypeFullChallenge        uint32 `json:"full-challenge"`

	PeerMessageTypeRelay uint32 `json:"relay"`
}

func (mp *MetricPool) handle(msg uint8) {
	if !mp.enabled {
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
	case PeerMessageTypeSnapshotAnnouncement:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotAnnouncement, 1)
	case PeerMessageTypeSnapshotCommitment:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotCommitment, 1)
	case PeerMessageTypeTransactionChallenge:
		atomic.AddUint32(&mp.PeerMessageTypeTransactionChallenge, 1)
	case PeerMessageTypeSnapshotResponse:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotResponse, 1)
	case PeerMessageTypeSnapshotFinalization:
		atomic.AddUint32(&mp.PeerMessageTypeSnapshotFinalization, 1)
	case PeerMessageTypeCommitments:
		atomic.AddUint32(&mp.PeerMessageTypeCommitments, 1)
	case PeerMessageTypeFullChallenge:
		atomic.AddUint32(&mp.PeerMessageTypeFullChallenge, 1)
	case PeerMessageTypeRelay:
		atomic.AddUint32(&mp.PeerMessageTypeRelay, 1)
	}
}

func (mp *MetricPool) String() string {
	b, err := json.Marshal(mp)
	if err != nil {
		panic(err)
	}
	return string(b)
}
