package main

import (
	"fmt"

	"github.com/ShivankSharma070/DistributedFileStorage/p2p"
)

func OnPeer(peer p2p.Peer) error {
	fmt.Println("Doing some logic computation with peer")
	peer.Close()
	return nil
}
func main() {
	opts := p2p.TCPTransportOpts{
		ListenAddr: ":3000",
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder : p2p.DefaultDecoder{},
		OnPeer : OnPeer,
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
