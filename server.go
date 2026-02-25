package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/ShivankSharma070/DistributedFileStorage/p2p"
)

func init() {
	gob.Register(MessageStoreFile{})
}

type FileServerOpts struct {
	StorageRoot       string
	PathTransformFunc PathTransformFunc
	Transport         p2p.Transport
	bootstrapNodes    []string
}

type FileServer struct {
	FileServerOpts

	peerLock sync.Mutex
	peers    map[string]p2p.Peer

	store  *Store
	quitCh chan struct{}
}

func NewFileServer(opts FileServerOpts) *FileServer {
	serverOpts := StoreOpts{
		PathTransformFunc: opts.PathTransformFunc,
		root:              opts.StorageRoot,
	}

	return &FileServer{
		store:          NewStore(serverOpts),
		FileServerOpts: opts,
		quitCh:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

func (s *FileServer) broadcast(data *Message) error {
	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	mw := io.MultiWriter(peers...)
	return gob.NewEncoder(mw).Encode(data)
}

type Message struct {
	Payload any
}

type MessageStoreFile struct {
	Key string 
	Size int64
}

// type DataMessage struct {
// 	Key  string
// 	Data []byte
// }

func (s *FileServer) StoreData(key string, r io.Reader) error {

	buf := new(bytes.Buffer)
	msg := Message{ 
		Payload: MessageStoreFile{
			Key: key,
			Size: 15, 
		},
	}

	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range s.peers {
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	time.Sleep(2 * time.Second)

	payload := []byte("VERY LARGE FILE")
	for _, peer := range s.peers {
		if err := peer.Send(payload); err != nil {
			return err
		}
	}

	// // Store the file on disk
	// buf := new(bytes.Buffer)
	// tee := io.TeeReader(r, buf)
	// if err := s.store.Write(key, tee); err != nil {
	// 	return err
	// }
	//
	// // Broadcast it to all other known peers
	// p := &DataMessage{
	// 	Key:  key,
	// 	Data: buf.Bytes(),
	// }
	//
	// fmt.Println(buf.Bytes())
	// return s.broadcast(&Message{
	// 	From:    "todo",
	// 	Payload: p,
	// })
	return nil
}

func (s *FileServer) OnPeer(p p2p.Peer) error {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	s.peers[p.RemoteAddr().String()] = p
	if !p.(*p2p.TCPPeer).Outbound {
		log.Printf("Connected with remote(server - %s): %s\n", s.Transport.(*p2p.TCPTransport).ListenAddr, p.RemoteAddr())
	}
	return nil
}

func (s *FileServer) Start() error {
	if err := s.Transport.ListenAndAccept(); err != nil {
		return err
	}

	s.BootStrapNetwork()
	s.loop()

	return nil
}

func (s *FileServer) Quit() {
	close(s.quitCh)
}

func (s *FileServer) BootStrapNetwork() error {
	for _, addr := range s.bootstrapNodes {
		if len(addr) == 0 {
			continue
		}
		go func(addr string) {
			log.Printf("Server %s is trying to connect to %s\n", s.Transport.(*p2p.TCPTransport).ListenAddr, addr)
			err := s.Transport.Dial(addr)
			if err != nil {
				log.Printf("Dial Error: %s\n", err)
			}
		}(addr)
	}
	return nil
}


func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return s.handleMessageStoreFile(from, v) 
	}
	return nil
}

func (s *FileServer) handleMessageStoreFile (from string, msg MessageStoreFile) error{ 
	peer , ok := s.peers[from]
	if !ok {
		return fmt.Errorf("Peer %s could not be found in peer list", from)
	}

	if err := s.store.Write(msg.Key, io.LimitReader(peer, msg.Size)) ; err != nil {
		return  err
	}


	peer.(*p2p.TCPPeer).Wg.Done()

	return nil
}


func (s *FileServer) loop() {
	defer func() {
		log.Println("File server stopped due to user action.")
		s.Transport.Close()
	}()
	for {
		select {
		case <-s.quitCh:
			return
		case rpc := <-s.Transport.Consume():
			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Fatal(err)
			}

			if err := s.handleMessage(rpc.From, &msg); err != nil {
				log.Fatal(err)
			}

		}
	}
}
