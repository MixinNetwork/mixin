package network

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

// /etc/sysctl.conf
// net.core.rmem_max=8388608
// net.core.wmem_max=8388608

const (
	MaxIncomingStreams = 128
	HandshakeTimeout   = 10 * time.Second
	IdleTimeout        = 60 * time.Second
	ReadDeadline       = 10 * time.Second
	WriteDeadline      = 10 * time.Second
)

type QuicClient struct {
	session quic.Connection
	send    quic.SendStream
	receive quic.ReceiveStream
}

type QuicTransport struct {
	addr     string
	tls      *tls.Config
	listener *quic.Listener
}

func NewQuicServer(addr string) (*QuicTransport, error) {
	tlsConf := generateTLSConfig()
	return &QuicTransport{
		addr: addr,
		tls:  tlsConf,
	}, nil
}

func NewQuicClient(addr string) (*QuicTransport, error) {
	return &QuicTransport{
		addr: addr,
		tls: &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"mixin-quic-peer"},
		},
	}, nil
}

func (t *QuicTransport) Dial(ctx context.Context) (Client, error) {
	sess, err := quic.DialAddr(ctx, t.addr, t.tls, &quic.Config{
		MaxIncomingStreams:   MaxIncomingStreams,
		HandshakeIdleTimeout: HandshakeTimeout,
		MaxIdleTimeout:       IdleTimeout,
		KeepAlivePeriod:      IdleTimeout / 2,
	})
	if err != nil {
		return nil, err
	}
	stm, err := sess.OpenUniStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &QuicClient{
		session: sess,
		send:    stm,
	}, nil
}

func (t *QuicTransport) Listen() error {
	l, err := quic.ListenAddr(t.addr, t.tls, &quic.Config{
		MaxIncomingStreams:   MaxIncomingStreams,
		HandshakeIdleTimeout: HandshakeTimeout,
		MaxIdleTimeout:       IdleTimeout,
		KeepAlivePeriod:      IdleTimeout / 2,
	})
	if err != nil {
		return err
	}
	t.listener = l
	return nil
}

func (t *QuicTransport) Close() error {
	return t.listener.Close()
}

func (t *QuicTransport) Accept(ctx context.Context) (Client, error) {
	sess, err := t.listener.Accept(ctx)
	if err != nil {
		return nil, err
	}
	stm, err := sess.AcceptUniStream(ctx)
	if err != nil {
		return nil, err
	}
	return &QuicClient{
		session: sess,
		receive: stm,
	}, nil
}

func (c *QuicClient) RemoteAddr() net.Addr {
	return c.session.RemoteAddr()
}

func (c *QuicClient) Receive() (*TransportMessage, error) {
	err := c.receive.SetReadDeadline(time.Now().Add(ReadDeadline))
	if err != nil {
		return nil, err
	}
	m := &TransportMessage{}
	header := make([]byte, TransportMessageHeaderSize)
	s, err := c.receive.Read(header)
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
	if m.Size > TransportMessageMaxSize {
		return nil, fmt.Errorf("quic receive invalid message size %d", m.Size)
	}
	for {
		data := make([]byte, int(m.Size)-len(m.Data))
		s, err = c.receive.Read(data)
		if err != nil {
			return nil, err
		}
		m.Data = append(m.Data, data[:s]...)
		if len(m.Data) == int(m.Size) {
			break
		}
	}

	return m, err
}

func (c *QuicClient) Send(data []byte) error {
	if l := len(data); l < 1 || l > TransportMessageMaxSize {
		return fmt.Errorf("quic send invalid message size %d", l)
	}

	err := c.send.SetWriteDeadline(time.Now().Add(WriteDeadline))
	if err != nil {
		return err
	}
	header := []byte{TransportMessageVersion, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[2:], uint32(len(data)))
	_, err = c.send.Write(header)
	if err != nil {
		return err
	}
	_, err = c.send.Write(data)
	return err
}

func (c *QuicClient) Close() error {
	if c.send != nil {
		c.send.Close()
	}
	return c.session.CloseWithError(0, "DONE")
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
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
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
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
