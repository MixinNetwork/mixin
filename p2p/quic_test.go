package p2p

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuic(t *testing.T) {
	require := require.New(t)

	serverTrans, err := NewQuicRelayer("127.0.0.1:0")
	require.Nil(err)
	require.NotNil(serverTrans)
	defer serverTrans.Close()

	listenAddr := serverTrans.listener.Addr().String()

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

	client, err := NewQuicConsumer(context.Background(), listenAddr)
	require.Nil(err)
	require.NotNil(client)
	require.NotNil(client.RemoteAddr())
	err = client.Send([]byte("hello mixin"))
	require.Nil(err)
	<-wait
	err = client.Close("done")
	require.Nil(err)
}

func TestQuicErrors(t *testing.T) {
	require := require.New(t)

	_, err := NewQuicRelayer("bad-addr")
	require.Error(err)

	_, err = NewQuicConsumer(context.Background(), "bad-addr")
	require.Error(err)

	err = (&QuicClient{}).Send(nil)
	require.ErrorContains(err, "invalid message size 0")
	err = (&QuicClient{}).Send(bytes.Repeat([]byte{1}, TransportMessageMaxSize+1))
	require.ErrorContains(err, "invalid message size")

	closedRelayer, err := NewQuicRelayer("127.0.0.1:0")
	require.Nil(err)
	require.NoError(closedRelayer.Close())
	_, err = closedRelayer.Accept(context.Background())
	require.ErrorContains(err, "quic.Accept")

	serverTrans, err := NewQuicRelayer("127.0.0.1:0")
	require.Nil(err)
	require.NotNil(serverTrans)
	defer serverTrans.Close()

	accept := make(chan *QuicClient, 1)
	go func() {
		server, err := serverTrans.Accept(context.Background())
		require.Nil(err)
		accept <- server.(*QuicClient)
	}()

	client, err := NewQuicConsumer(context.Background(), serverTrans.listener.Addr().String())
	require.Nil(err)
	require.Nil(client.Send([]byte("accept")))
	server := <-accept
	_, err = server.stream.Write([]byte{0, 0, 0, 0, 0, 0})
	require.Nil(err)
	_, err = client.Receive()
	require.ErrorContains(err, "invalid message version")
	require.Nil(client.Close("version"))

	serverTrans2, err := NewQuicRelayer("127.0.0.1:0")
	require.Nil(err)
	require.NotNil(serverTrans2)
	defer serverTrans2.Close()

	accept2 := make(chan *QuicClient, 1)
	go func() {
		server, err := serverTrans2.Accept(context.Background())
		require.Nil(err)
		accept2 <- server.(*QuicClient)
	}()

	client2, err := NewQuicConsumer(context.Background(), serverTrans2.listener.Addr().String())
	require.Nil(err)
	require.Nil(client2.Send([]byte("accept")))
	server2 := <-accept2
	_, err = server2.stream.Write([]byte{TransportMessageVersion, 0, 0xff, 0xff, 0xff, 0xff})
	require.Nil(err)
	_, err = client2.Receive()
	require.ErrorContains(err, "invalid message size")
	require.Nil(client2.Close("size"))
}
