package network

import (
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto/ed25519"
	"github.com/stretchr/testify/assert"
)

func init() {
	ed25519.Load()
}

func TestQuic(t *testing.T) {
	assert := assert.New(t)

	addr := "127.0.0.1:7000"
	serverTrans, err := NewQuicServer(addr)
	assert.Nil(err)
	assert.NotNil(serverTrans)
	defer serverTrans.Close()
	err = serverTrans.Listen()
	assert.Nil(err)
	go func() {
		server, err := serverTrans.Accept()
		assert.Nil(err)
		assert.NotNil(server)
		msg, err := server.Receive()
		assert.Nil(err)
		assert.Equal("hello mixin", string(msg))
	}()

	clientTrans, err := NewQuicClient(addr)
	assert.Nil(err)
	assert.NotNil(clientTrans)
	client, err := clientTrans.Dial()
	assert.Nil(err)
	assert.NotNil(client)
	err = client.Send([]byte("hello mixin"))
	assert.Nil(err)
	time.Sleep(1 * time.Second)
}
