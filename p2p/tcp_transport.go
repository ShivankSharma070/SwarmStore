package p2p

import (
	"fmt"
	"net"
	"reflect"
)

// TCPPeer represents the remote node over the established TCP connection
type TCPPeer struct {
	// conn is the underlying connection of the peer
	conn net.Conn

	// If we dial and retreive a conn: outbound : true
	// If we accept and retreive a conn: outbound : false
	outbound bool
}

func (p *TCPPeer) Close() error {
	return p.conn.Close()
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		conn:     conn,
		outbound: outbound,
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
		rpcChan:          make(chan RPC),
	}
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.ListenAddr)
	if err != nil {
		return err
	}

	fmt.Println("Server Started")

	go t.startAcceptLoop()
	return nil
}

func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcChan
}

func (t *TCPTransport) startAcceptLoop() {
	for {

		conn, err := t.listener.Accept()
		if err != nil {
			fmt.Printf("Tcp connection error: %s\n", err)
		}

		fmt.Printf("Got new Connection: %+v\n", conn)
		go t.handleConnection(conn)
	}
}

func (t *TCPTransport) handleConnection(conn net.Conn) {
	var err error
	defer func() {
		fmt.Printf("Dropping connnection: %s\n", err)
		conn.Close()
	}()
	peer := NewTCPPeer(conn, true)

	if err = t.HandshakeFunc(peer); err != nil {
		fmt.Printf("Tcp error : %s\n", err)
		return
	}

	if t.OnPeer != nil {
		if err = t.OnPeer(peer); err != nil {
			return
		}
	}

	rpc := RPC{}
	// Read loop
	for {
		err := t.Decoder.Decode(conn, &rpc)
		if reflect.TypeOf(err) == reflect.TypeOf(&net.OpError{}) { 
			return
		}
		if err != nil {
			fmt.Printf("Tcp read error : %s\n", err)
			continue
		}

		rpc.From = conn.RemoteAddr()
		t.rpcChan <- rpc
	}
}
