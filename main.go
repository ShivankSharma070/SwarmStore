package main

import (
	"bytes"
	"log"
	"strings"
	"time"

	"github.com/ShivankSharma070/DistributedFileStorage/p2p"
)

func makeServer(listenAddr string, node ...string) *FileServer {

	tcp_opts := p2p.TCPTransportOpts{
		ListenAddr:    listenAddr,
		HandshakeFunc: p2p.NOPHandshakeFunc,
		Decoder:       p2p.DefaultDecoder{},
		// TODO: OnPeer
	}

	tcp_transport := p2p.NewTCPTransport(tcp_opts)
	opts := FileServerOpts{
		StorageRoot:       strings.TrimPrefix(listenAddr, ":") + "_network",
		PathTransformFunc: CASPathTransformFunc,
		Transport:         tcp_transport,
		bootstrapNodes:    node,
	}

	s := NewFileServer(opts)
	tcp_transport.OnPeer = s.OnPeer
	return s
}

func main() {
	s1 := makeServer(":3000", "")
   s2 := makeServer(":4000", ":3000")

	go func() {
		log.Fatal(s1.Start())
	}()

	time.Sleep(time.Second * 1)

	go func() {
		log.Fatal(s2.Start())
	}()


	time.Sleep(time.Second * 1)
	buf := bytes.NewReader([]byte("Very large file"))
	s2.StoreData("mysecretkey", buf)

	select {}
}
