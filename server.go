package main

import (
	"bytes"
	"encoding/binary"
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
	gob.Register(MessageGetFile{})
}

type FileServerOpts struct {
	EncryptionKey     []byte
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

func (s *FileServer) broadcast(msg *Message) error {
	buff := new(bytes.Buffer)
	if err := gob.NewEncoder(buff).Encode(msg); err != nil {
		return err
	}

	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingMessage})
		if err := peer.Send(buff.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

type Message struct {
	Payload any
}

type MessageStoreFile struct {
	Key  string
	Size int64
}

type MessageGetFile struct {
	Key string
}

func (s *FileServer) Get(key string) (io.Reader, error) {
	if s.store.Has(key) {
		_, buf, err := s.store.Read(key)
		if err != nil {
			return nil, err
		}
		return buf, nil
	}
	log.Printf(
		"[%s] Dont have file locally, fetching from network",
		s.Transport.Addr())

	msg := &Message{
		Payload: MessageGetFile{
			Key: key,
		},
	}

	if err := s.broadcast(msg); err != nil {
		return nil, err
	}

	time.Sleep(time.Millisecond * 100)

	for _, peer := range s.peers {
		var fSize int64
		binary.Read(peer, binary.LittleEndian, &fSize)

		n, err := s.store.writeDecrypt(s.EncryptionKey, key, io.LimitReader(peer, fSize))
		if err != nil {
			return nil, err
		}
		fmt.Printf(
			"[%s] Received %d bytes over the network.\n",
			s.Transport.Addr(),
			n)
		peer.CloseStream()
	}

	_, buf, err := s.store.Read(key)
	return buf, err
}

func (s *FileServer) Store(key string, r io.Reader) error {
	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	size, err := s.store.Write(key, tee)
	if err != nil {
		return err
	}

	msg := &Message{
		Payload: MessageStoreFile{
			Key:  key,
			Size: size + 16,
		},
	}

	if err := s.broadcast(msg); err != nil {
		return err
	}

	time.Sleep(5 * time.Millisecond)

	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}

	// Mulitwriter enables writting to mulitple peers, for loop cannot be used 
	// here as reader becomes unreadable once it is read
	mw := io.MultiWriter(peers...)
	mw.Write([]byte{p2p.IncomingStream})
	n, err := copyEncrypt(s.EncryptionKey, fileBuffer, mw)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] Broadcasted %d bytes \n", s.Transport.Addr(), n) 

	return nil
}

func (s *FileServer) OnPeer(p p2p.Peer) error {
	s.peerLock.Lock()
	defer s.peerLock.Unlock()
	s.peers[p.RemoteAddr().String()] = p
	if !p.(*p2p.TCPPeer).Outbound {
		log.Printf(
			"Connected with remote(server - %s): %s\n",
			s.Transport.(*p2p.TCPTransport).ListenAddr,
			p.RemoteAddr())
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
			log.Printf(
				"Server %s is trying to connect to %s\n",
				s.Transport.(*p2p.TCPTransport).ListenAddr,
				addr)
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
	case MessageGetFile:
		return s.handleMessageGetFile(from, v)
	}
	return nil
}

func (s *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	if !s.store.Has(msg.Key) {
		return fmt.Errorf(
			"[%s] need to serve file (%s), but it does not exists on disk",
			s.Transport.Addr(),
			msg.Key)
	}

	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("Peer %s could not be found in peer list", from)
	}

	fmt.Printf("[%s] Serve file %s over the network\n", s.Transport.Addr(), msg.Key)
	fSize, r, err := s.store.Read(msg.Key)
	if err != nil {
		return err
	}

	rc, ok := r.(io.ReadCloser)
	if ok {
		defer func(rc io.ReadCloser) {
			fmt.Println("Closing reader")
			rc.Close()
		}(rc)
	}

	peer.Send([]byte{p2p.IncomingStream})
	binary.Write(peer, binary.LittleEndian, fSize)
	n, err := io.Copy(peer, r)
	if err != nil {
		return err
	}

	log.Printf("[%s] Written %d bytes, to the network %s", s.Transport.Addr(), n, from)
	return nil
}

func (s *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("Peer %s could not be found in peer list", from)
	}

	n, err := s.store.Write(msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}

	log.Printf("(%s) -> Written (%d) bytes to disk\n", s.Transport.Addr(), n)
	peer.CloseStream()
	return nil
}

func (s *FileServer) loop() {
	defer func() {
		log.Println("File server stopped due to error or user action.")
		s.Transport.Close()
	}()
	for {
		select {
		case <-s.quitCh:
			return
		case rpc := <-s.Transport.Consume():
			var msg Message
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Println("Decoding error: ", err)
			}

			if err := s.handleMessage(rpc.From, &msg); err != nil {
				log.Println("Handle message error: ", err)
			}
		}
	}
}
