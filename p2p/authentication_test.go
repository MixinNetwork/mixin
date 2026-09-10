package p2p

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/stretchr/testify/require"
)

func TestAuthenticateRelayer(t *testing.T) {
	for _, variant := range []string{"valid", "wrong identity", "not a relayer", "invalid authentication", "missing reply", "wrong message", "send failure"} {
		t.Run(variant, func(t *testing.T) {
			handle := newP2PStubHandle(t)
			t.Cleanup(handle.cache.Close)
			me := NewPeer(handle, crypto.Blake3Hash([]byte("authentication client")), "test", false)
			serverID := crypto.Blake3Hash([]byte("expected authentication server"))
			relayer := NewPeer(nil, serverID, "test relayer", true)
			handle.authToken.PeerId = serverID
			handle.authToken.IsRelayer = true
			client := &scriptedClient{receiveSteps: []receiveStep{{msg: &TransportMessage{
				Version: TransportMessageVersion,
				Data:    buildAuthenticationMessage(handle.BuildAuthenticationMessage(me.IdForNetwork)),
			}}}}
			switch variant {
			case "wrong identity":
				handle.authToken.PeerId = crypto.Blake3Hash([]byte("unexpected authentication server"))
			case "not a relayer":
				handle.authToken.IsRelayer = false
			case "invalid authentication":
				handle.authErr = errors.New("invalid authentication signature")
			case "missing reply":
				client.receiveSteps = nil
			case "wrong message":
				client.receiveSteps[0].msg.Data = buildGraphMessage(handle)
			case "send failure":
				client.sendErrAt = 1
				client.sendErr = io.ErrClosedPipe
			}
			err := me.authenticateRelayer(relayer, client)
			require.Len(t, client.sent, 1)
			require.Equal(t, buildAuthenticationMessage(handle.BuildAuthenticationMessage(serverID)), client.sent[0])
			if variant == "valid" {
				require.NoError(t, err)
				require.Equal(t, serverID, relayer.authentication.PeerId)
				require.Equal(t, handle.key.Public(), relayer.authentication.PublicSpendKey)
			} else {
				require.Error(t, err)
				require.Nil(t, relayer.authentication)
			}
		})
	}
}

func TestAuthenticateNeighborRejectsFailedReply(t *testing.T) {
	handle := newP2PStubHandle(t)
	t.Cleanup(handle.cache.Close)
	me := NewPeer(handle, crypto.Blake3Hash([]byte("authentication server")), "test", true)
	client := &scriptedClient{
		receiveSteps: []receiveStep{{msg: &TransportMessage{
			Version: TransportMessageVersion,
			Data:    buildAuthenticationMessage(handle.BuildAuthenticationMessage(me.IdForNetwork)),
		}}},
		sendErrAt: 1,
		sendErr:   io.ErrClosedPipe,
	}
	peer, err := me.authenticateNeighbor(client)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.Nil(t, peer)
}

func TestConnectRelayerRejectsUnexpectedIdentity(t *testing.T) {
	handle := newP2PStubHandle(t)
	t.Cleanup(handle.cache.Close)
	handle.authToken.IsRelayer = true
	me := NewPeer(handle, crypto.Blake3Hash([]byte("rejected authentication client")), "test", false)
	serverID := crypto.Blake3Hash([]byte("expected authentication server"))
	server, err := NewQuicRelayer("127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, server.Close()) })
	serverDone := make(chan error, 1)
	go func() {
		client, err := server.Accept(context.Background())
		if err != nil {
			serverDone <- err
			return
		}
		defer client.Close("authentication test server")
		_, err = client.Receive()
		if err == nil {
			err = client.Send(buildAuthenticationMessage(handle.BuildAuthenticationMessage(me.IdForNetwork)))
		}
		if err == nil {
			// Keep the reply stream open until the rejecting client closes it.
			_, err = client.Receive()
		}
		serverDone <- err
	}()
	relayer := NewPeer(nil, serverID, server.listener.Addr().String(), true)
	connectDone := make(chan error, 1)
	go func() { connectDone <- me.connectRelayer(relayer) }()
	select {
	case err := <-connectDone:
		require.ErrorContains(t, err, "relayer authentication id mismatch")
	case <-time.After(3 * time.Second):
		t.Fatal("failed authentication did not close the connection")
	}
	require.Nil(t, me.relayers.Get(serverID))
	require.Nil(t, relayer.authentication)
	select {
	case err := <-serverDone:
		require.Error(t, err, "rejected connection must be closed")
	case <-time.After(3 * time.Second):
		t.Fatal("server did not observe the rejected connection closing")
	}
}
