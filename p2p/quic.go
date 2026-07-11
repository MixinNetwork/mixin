package p2p

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/MixinNetwork/mixin/crypto"
	"github.com/quic-go/quic-go"
)

// /etc/sysctl.conf
// net.core.rmem_max=8388608
// net.core.wmem_max=8388608

const (
	// Each peer connection uses exactly one bidirectional stream.
	MaxIncomingStreams = 1
	HandshakeTimeout   = 10 * time.Second
	IdleTimeout        = 60 * time.Second
	WriteDeadline      = 10 * time.Second
	ReadDeadline       = 2 * WriteDeadline
)

type QuicClient struct {
	session *quic.Conn
	stream  *quic.Stream
	close   sync.Once
}

type QuicRelayer struct {
	addr     string
	listener *quic.Listener
}

func NewQuicRelayer(listenAddr string) (*QuicRelayer, error) {
	tls := generateTLSConfig()
	l, err := quic.ListenAddr(listenAddr, tls, &quic.Config{
		MaxIncomingStreams:   MaxIncomingStreams,
		HandshakeIdleTimeout: HandshakeTimeout,
		MaxIdleTimeout:       IdleTimeout,
		KeepAlivePeriod:      0,
	})
	if err != nil {
		return nil, err
	}
	return &QuicRelayer{
		addr:     listenAddr,
		listener: l,
	}, nil
}

func NewQuicConsumer(ctx context.Context, relayer string) (*QuicClient, error) {
	sess, err := quic.DialAddr(ctx, relayer, &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"mixin-quic-peer"},
	}, &quic.Config{
		MaxIncomingStreams:   MaxIncomingStreams,
		HandshakeIdleTimeout: HandshakeTimeout,
		MaxIdleTimeout:       IdleTimeout,
		KeepAlivePeriod:      IdleTimeout / 2,
	})
	if err != nil {
		return nil, fmt.Errorf("quic.DialAddr(%s) => %v", relayer, err)
	}
	stm, err := sess.OpenStreamSync(ctx)
	if err != nil {
		_ = sess.CloseWithError(0, "open stream failed")
		return nil, fmt.Errorf("quic.OpenStreamSync(%s, %v) => %v", relayer, sess, err)
	}
	return &QuicClient{
		session: sess,
		stream:  stm,
	}, nil
}

func (t *QuicRelayer) Close() error {
	return t.listener.Close()
}

func (t *QuicRelayer) Accept(ctx context.Context) (Client, error) {
	sess, err := t.acceptConnection(ctx)
	if err != nil {
		return nil, err
	}
	return acceptQuicClient(ctx, sess)
}

func (t *QuicRelayer) acceptConnection(ctx context.Context) (*quic.Conn, error) {
	sess, err := t.listener.Accept(ctx)
	if err != nil {
		return nil, fmt.Errorf("quic.Accept() => %v", err)
	}
	return sess, nil
}

func acceptQuicClient(ctx context.Context, sess *quic.Conn) (*QuicClient, error) {
	streamContext, cancel := context.WithTimeout(ctx, HandshakeTimeout)
	defer cancel()
	stm, err := sess.AcceptStream(streamContext)
	if err != nil {
		_ = sess.CloseWithError(0, "accept stream failed")
		return nil, fmt.Errorf("quic.AcceptStream(%v) => %v", sess, err)
	}
	return &QuicClient{
		session: sess,
		stream:  stm,
	}, nil
}

func (c *QuicClient) RemoteAddr() net.Addr {
	return c.session.RemoteAddr()
}

func (c *QuicClient) Receive() (*TransportMessage, error) {
	return c.receiveWithLimit(TransportMessageMaxSize)
}

func (c *QuicClient) receiveWithLimit(maxSize uint32) (*TransportMessage, error) {
	if maxSize == 0 || maxSize > TransportMessageMaxSize {
		return nil, fmt.Errorf("quic receive invalid size limit %d", maxSize)
	}
	err := c.stream.SetReadDeadline(time.Now().Add(ReadDeadline))
	if err != nil {
		return nil, err
	}
	m := &TransportMessage{}
	header := make([]byte, TransportMessageHeaderSize)
	s, err := io.ReadFull(c.stream, header)
	if err != nil {
		return nil, err
	}
	if s != TransportMessageHeaderSize {
		return nil, fmt.Errorf("quic receive invalid message header size %d", s)
	}
	m.Version = header[0]
	if m.Version != TransportMessageVersion {
		return nil, fmt.Errorf("quic receive invalid message version %d", m.Version)
	}
	m.Size = binary.BigEndian.Uint32(header[2:])
	if m.Size > maxSize {
		return nil, fmt.Errorf("quic receive invalid message size %d", m.Size)
	}

	m.Data = make([]byte, m.Size)
	_, err = io.ReadFull(c.stream, m.Data)
	return m, err
}

func (c *QuicClient) Send(data []byte) error {
	if l := len(data); l < 1 || l > TransportMessageMaxSize {
		return fmt.Errorf("quic send invalid message size %d", l)
	}

	err := c.stream.SetWriteDeadline(time.Now().Add(WriteDeadline))
	if err != nil {
		return err
	}
	header := []byte{TransportMessageVersion, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[2:], uint32(len(data)))
	n, err := c.stream.Write(header)
	if err != nil {
		return err
	}
	if n != len(header) {
		return io.ErrShortWrite
	}
	n, err = c.stream.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}

func (c *QuicClient) Close(code string) {
	c.close.Do(func() {
		_ = c.stream.Close()
		_ = c.session.CloseWithError(0, code)
	})
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(crypto.RandReader(), 2048)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour * 24 * 30),
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(crypto.RandReader(), &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"mixin-quic-peer"},
	}
}
