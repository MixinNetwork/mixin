package network

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuic(t *testing.T) {
	require := require.New(t)

	addr := "127.0.0.1:7000"
	serverTrans, err := NewQuicServer(addr)
	require.Nil(err)
	require.NotNil(serverTrans)
	defer serverTrans.Close()
	err = serverTrans.Listen()
	require.Nil(err)

	wait := make(chan struct{})
	go func() {
		server, err := serverTrans.Accept(context.Background())
		require.Nil(err)
		require.NotNil(server)
		msg, err := server.Receive()
		require.Nil(err)
		require.Equal("hello mixin", string(msg.Data))
		wait <- struct{}{}
	}()

	clientTrans, err := NewQuicClient(addr)
	require.Nil(err)
	require.NotNil(clientTrans)
	client, err := clientTrans.Dial(context.Background())
	require.Nil(err)
	require.NotNil(client)
	err = client.Send([]byte("hello mixin"))
	require.Nil(err)
	<-wait
}
