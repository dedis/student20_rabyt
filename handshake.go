package routing

import (
	"go.dedis.ch/dela/mino"
)

type Handshake struct {
	idBase uint8
	// sorted array of ids
	ids       []Id
	addresses []mino.Address
}

func NewHandshake(idBase uint8, ids []Id, addresses []mino.Address) Handshake {
	return Handshake{idBase: idBase, ids: ids, addresses: addresses}
}