package api

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"

	"github.com/MixinNetwork/mobilecoin-account/base58"
	"github.com/MixinNetwork/mobilecoin-account/types"
	"google.golang.org/protobuf/proto"
)

type PublicAddress struct {
	ViewPublicKey   string `json:"view_public_key"`
	SpendPublicKey  string `json:"spend_public_key"`
	FogReportUrl    string `json:"fog_report_url"`
	FogReportId     string `json:"fog_report_id"`
	FogAuthoritySig string `json:"fog_authority_sig"`
}

func (addr *PublicAddress) B58Code() (string, error) {
	view, err := hex.DecodeString(addr.ViewPublicKey)
	if err != nil {
		return "", err
	}
	spend, err := hex.DecodeString(addr.SpendPublicKey)
	if err != nil {
		return "", err
	}
	sig, err := hex.DecodeString(addr.FogAuthoritySig)
	if err != nil {
		return "", err
	}
	address := &types.PublicAddress{
		ViewPublicKey:   &types.CompressedRistretto{Data: view},
		SpendPublicKey:  &types.CompressedRistretto{Data: spend},
		FogReportUrl:    addr.FogReportUrl,
		FogReportId:     addr.FogReportId,
		FogAuthoritySig: sig,
	}
	wrapper := &types.PrintableWrapper_PublicAddress{PublicAddress: address}
	data, err := proto.Marshal(&types.PrintableWrapper{Wrapper: wrapper})
	if err != nil {
		return "", err
	}

	bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(bytes, crc32.ChecksumIEEE(data))
	bytes = append(bytes, data...)
	return base58.Encode(bytes), nil
}

func DecodeB58Code(account string) (*PublicAddress, error) {
	data := base58.Decode(account)
	if len(data) < 4 {
		return nil, fmt.Errorf("Invalid account %s", account)
	}
	sum := make([]byte, 4)
	binary.LittleEndian.PutUint32(sum, crc32.ChecksumIEEE(data[4:]))
	if bytes.Compare(sum, data[:4]) != 0 {
		return nil, fmt.Errorf("Invalid account %s", account)
	}
	var wrapper types.PrintableWrapper
	err := proto.Unmarshal(data[4:], &wrapper)
	if err != nil {
		return nil, err
	}
	address := wrapper.GetPublicAddress()

	return &PublicAddress{
		ViewPublicKey:   hex.EncodeToString(address.GetViewPublicKey().GetData()),
		SpendPublicKey:  hex.EncodeToString(address.GetSpendPublicKey().GetData()),
		FogReportUrl:    address.GetFogReportUrl(),
		FogReportId:     address.GetFogReportId(),
		FogAuthoritySig: hex.EncodeToString(address.GetFogAuthoritySig()),
	}, nil
}

// TODO decrapted
func DecodeAccount(account string) (*PublicAddress, error) {
	return DecodeB58Code(account)
}
