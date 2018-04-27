package main

import (
	"context"
	"flag"
	"log"

	"mixin.one/blockchain/external"
	"mixin.one/durable"
	"mixin.one/session"
)

const spannerName = ""

func main() {
	blockHash := flag.String("block", "", "scan a single block")
	tokenAddress := flag.String("token", "", "add a token to database")
	tokenName := flag.String("name", "", "token name")
	tokenSymbol := flag.String("symbol", "", "token symbol")
	tokenDecimals := flag.Int64("decimals", 0, "token decimals")
	flag.Parse()

	spanner, err := durable.OpenSpannerClient(context.Background(), spannerName)
	if err != nil {
		log.Println(err)
	}
	defer spanner.Close()
	db := durable.WrapDatabase(spanner, nil)
	ctx := session.WithDatabase(context.Background(), db)

	if *blockHash != "" {
		rpc, err := NewRPC()
		if err != nil {
			log.Println(err)
		}
		block, err := rpc.GetBlockByHash(ctx, *blockHash)
		if err != nil {
			log.Println(err)
		}
		for _, tx := range block.Transactions {
			log.Println(*tx)
		}
	} else if *tokenAddress != "" {
		chainId := external.EthereumChainId
		token := &EthereumToken{
			Chain:    chainId,
			Id:       external.UniqueAssetId(chainId, *tokenAddress),
			Address:  *tokenAddress,
			Symbol:   *tokenSymbol,
			Name:     *tokenName,
			Decimals: *tokenDecimals,
		}
		err = persistWriteToken(ctx, token)
		if err != nil {
			log.Println(err)
		}
	} else {
		err = StartHTTP(spanner)
		if err != nil {
			log.Println(err)
		}
	}
}
