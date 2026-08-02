// Copyright (c) 2017-2022, The Monero Project
// Copyright (c) 2026, The Mixin Network
//
// Portions of this package are derived from Monero. The original copyright
// notice and BSD 3-Clause terms are retained in LICENSE.monero.

// Package bulletproofs implements Monero-compatible Bulletproofs+ aggregated
// 64-bit range proofs.
//
// The construction and wire conventions follow Monero v0.18.5.1. In
// particular, commitments use C = blind*G + value*H, proof points are encoded
// after multiplication by 1/8, generators and Fiat-Shamir challenges use
// legacy Keccak-256, and one proof may aggregate at most 16 commitments.
//
// This package has not received an independent cryptographic audit. Consensus
// integration must not be enabled until the implementation and its use in the
// transaction balance equation have been reviewed.
package bulletproofs
