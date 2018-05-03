package external

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestState(t *testing.T) {
	assert := assert.New(t)

	state := TransactionState(EthereumChainId, 35)
	assert.Equal(TransactionStatePending, state)

	state = TransactionState(EthereumChainId, 36)
	assert.Equal(TransactionStateConfirmed, state)

	state = TransactionState(EthereumChainId, 37)
	assert.Equal(TransactionStateConfirmed, state)
}
