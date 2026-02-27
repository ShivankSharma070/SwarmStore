package p2p

import (
	"encoding/gob"
	"io"
)

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type GOBDecoder struct{}

func (dec GOBDecoder) Decode(r io.Reader, v *RPC) error {
	return gob.NewDecoder(r).Decode(v)
}

type DefaultDecoder struct{}

func (dec DefaultDecoder) Decode(r io.Reader, msg *RPC) error {
	peekBuf := make([]byte, 1)
	if _, err := r.Read(peekBuf); err != nil { 
		return nil
	}

	// In case of stream we are not decoding  what is being send over the network.
	// We are just setting stream as true so that we can handle that in our logic
	stream := peekBuf[0] == IncomingStream
	if stream {
		msg.Stream = true
		return nil
	}

	buff := make([]byte, 1024)
	n, err := r.Read(buff)
	if err != nil {
		return err
	}

	msg.Payload = buff[:n]

	return nil
}
