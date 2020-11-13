package routing

import (
	"go.dedis.ch/dela/mino"
)

type RoutingTable struct {
	NextHop map[[]byte]mino.Address
}

type Router struct {

}