package main

import (
	"fmt"

	"github.com/ShivankSharma070/DistributedFileStorage/p2p"
)

func main() {
	opts := p2p.TCPTransportOpts{
		ListenAddr: ":3000",
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder : p2p.DefaultDecoder{},
		OnPeer : func(p2p.Peer) error { return fmt.Errorf("Error in onPeer func")},
	}

	tr := p2p.NewTCPTransport(opts)

	go func () {
		for {
			msg := <-tr.Consume()
			fmt.Printf("%+v\n", msg)
		}
	}()
	if err := tr.ListenAndAccept(); err != nil {
		fmt.Printf("TCP Error : %s\n", err)
	}

	select{}

}
