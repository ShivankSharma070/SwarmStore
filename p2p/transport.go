package p2p

import "net"

// Peer is an interface that represents remote node
type Peer interface {
	Send([]byte) error
	net.Conn
	CloseStream()
}

// Tranport is anything that handles the communication between
// the node in the network. This can be tcp, udp, websockets... etc
type Transport interface {
	Dial(string) error
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
	Addr() string
}
