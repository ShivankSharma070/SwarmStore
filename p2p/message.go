package p2p


// RPC holds any arbitary data that is being sent over the each transport between
// two nodes in the network
type RPC struct {
	From string
	Payload []byte
}
