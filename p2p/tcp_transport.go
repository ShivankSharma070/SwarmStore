package p2p

import (
	"errors"
	"fmt"
	"log"
	"net"
	"reflect"
	"sync"
)

// TCPPeer represents the remote node over the established TCP connection
type TCPPeer struct {
	// conn is the underlying connection of the peer
	net.Conn

	// If we dial and retreive a conn: outbound : true
	// If we accept and retreive a conn: outbound : false
	Outbound bool

	wg *sync.WaitGroup
}

func (p *TCPPeer) CloseStream() {
	p.wg.Done()
}

func (p *TCPPeer) Send(b []byte) error {
	_, err := p.Write(b)
	return err
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		Outbound: outbound,
		wg:       &sync.WaitGroup{},
	}
}

type TCPTransportOpts struct {
	ListenAddr    string
	HandshakeFunc HandshakeFunc
	Decoder       Decoder
	OnPeer        func(Peer) error
}

type TCPTransport struct {
	TCPTransportOpts
	listener net.Listener
	rpcChan  chan RPC
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		TCPTransportOpts: opts,
		rpcChan:          make(chan RPC, 1024),
	}
}

// Dial implements Transport interface
func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}

	go t.handleConnection(conn, true)
	return nil
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}

	log.Printf("Server Started on port: %s\n", t.ListenAddr)

	go t.startAcceptLoop()
	return nil
}

// Close implement Transport interface
func (t *TCPTransport) Close() error {
	log.Println("Stoped TCP Listener")
	return t.listener.Close()
}

// Consume implement Transport interface. It returns a read-only channel
// that will be used to recieve messages from other peer on the network
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcChan
}

func (t *TCPTransport) startAcceptLoop() {
	for {

		conn, err := t.listener.Accept()
		if errors.Is(err, net.ErrClosed) {
			log.Println("Stopped accepting new connecton")
			return
		}
		if err != nil {
			log.Printf("Tcp connection error: %s\n", err)
		}

		go t.handleConnection(conn, false)
	}
}

func (t *TCPTransport) handleConnection(conn net.Conn, outbound bool) {
	var err error
	defer func() {
		fmt.Printf("Dropping connnection: %s\n", err)
		conn.Close()
	}()
	peer := NewTCPPeer(conn, outbound)

	if err = t.HandshakeFunc(peer); err != nil {
		fmt.Printf("Tcp error : %s\n", err)
		return
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	// Read loop
	for {
		rpc := RPC{}
		err := t.Decoder.Decode(conn, &rpc)
		if reflect.TypeOf(err) == reflect.TypeOf(&net.OpError{}) {
			// TODO: handle this error
			return
		}
		if err != nil {
			fmt.Printf("Tcp read error : %s\n", err)
			return
		}

		rpc.From = conn.RemoteAddr().String()
		if rpc.Stream {
			peer.wg.Add(1)

			fmt.Printf(
				"[%s] incoming stream, waiting ....\n",
				conn.RemoteAddr())

			peer.wg.Wait()

			fmt.Printf(
				"[%s] stream closed, resuming read loop ....\n",
				conn.RemoteAddr())

			continue
		}
		t.rpcChan <- rpc
	}
}

func (t *TCPTransport) Addr() string {
	return t.ListenAddr
}
