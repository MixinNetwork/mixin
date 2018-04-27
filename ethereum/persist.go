package main

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"
	"google.golang.org/grpc/codes"
	"mixin.one/session"
)

const tokens_DDL = `
CREATE TABLE tokens (
	token_id    STRING(36) NOT NULL,
	chain_id    STRING(36) NOT NULL,
	address     STRING(128) NOT NULL,
	symbol      STRING(512) NOT NULL,
	name        STRING(512) NOT NULL,
	decimals    INT64 NOT NULL,
	updated_at  TIMESTAMP NOT NULL,
) PRIMARY KEY(token_id);
`

func persistReadToken(ctx context.Context, tokenId string) (*EthereumToken, error) {
	row, err := session.Database(ctx).ReadRow(ctx, "tokens", spanner.Key{tokenId}, []string{"chain_id", "address", "symbol", "name", "decimals"}, "persistReadToken")
	if spanner.ErrCode(err) == codes.NotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var token = EthereumToken{Id: tokenId}
	err = row.Columns(&token.Chain, &token.Address, &token.Symbol, &token.Name, &token.Decimals)
	return &token, err
}

func persistWriteToken(ctx context.Context, token *EthereumToken) error {
	cols := []string{"token_id", "chain_id", "address", "symbol", "name", "decimals", "updated_at"}
	vals := []interface{}{token.Id, token.Chain, token.Address, token.Symbol, token.Name, token.Decimals, time.Now()}
	return session.Database(ctx).Apply(ctx, []*spanner.Mutation{spanner.InsertOrUpdate("tokens", cols, vals)}, "tokens", "INSERT", "persistWriteToken")
}
