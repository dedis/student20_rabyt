package routing

import (
	"github.com/dedis/student20_rabyt/id"
	"github.com/dedis/student20_rabyt/routing/handshake"
	"go.dedis.ch/dela"
	"go.dedis.ch/dela/mino"
	"go.dedis.ch/dela/mino/router"
	"go.dedis.ch/dela/mino/router/tree/types"
	"math/rand"
)

// Prefix-based routing implementation of router.RoutingTable.
// Each entry in the NextHop maps a prefix of this node's id (thisId) plus
// a differing next digit to the address, where a packet whose destination has
// this prefix should be routed.
// Players contains the addresses of other nodes that this node is aware of and
// is used to select an alternative next hop when the connection to the one
// specified in NextHop fails.
type RoutingTable struct {
	thisNode mino.Address
	thisId   id.ArrayNodeID
	NextHop  map[id.StringPrefix]mino.Address
	Players  []mino.Address
}

// Implements router.Router
type Router struct {
	packetFac    router.PacketFactory
	hsFac        router.HandshakeFactory
	routingTable router.RoutingTable
}

// NewRouter returns a new router.
func NewRouter(f mino.AddressFactory) *Router {
	fac := types.NewPacketFactory(f)
	hsFac := handshake.NewHandshakeFactory(f)
	r := Router{
		packetFac:    fac,
		hsFac:        hsFac,
		routingTable: nil,
	}

	return &r
}

// GetPacketFactory implements router.Router. It returns the packet factory.
func (r *Router) GetPacketFactory() router.PacketFactory {
	return r.packetFac
}

// GetHandshakeFactory implements router.Router. It returns the handshake
// factory.
func (r *Router) GetHandshakeFactory() router.HandshakeFactory {
	return r.hsFac
}

// New implements router.Router. It decides on the base and length of node
// ids based on the number of players and creates the routing table for the node
// that is booting the protocol.
func (r *Router) New(players mino.Players, thisAddress mino.Address) (
	router.RoutingTable, error) {
	addrs := make([]mino.Address, 0, players.Len())
	iter := players.AddressIterator()
	for iter.HasNext() {
		addrs = append(addrs, iter.GetNext())
	}

	base, length := id.BaseAndLenFromPlayers(len(addrs))
	table, err := NewTable(addrs, thisAddress,
		id.NewArrayNodeID(thisAddress, base, length))
	if err != nil {
		return nil, err
	}
	r.routingTable = table
	return r.routingTable, nil
}

// GenerateTableFrom implements router.Router. It selects entries for the
// routing table from the addresses, received in the handshake.
func (r *Router) GenerateTableFrom(h router.Handshake) (router.RoutingTable,
	error) {
	hs := h.(handshake.Handshake)
	dela.Logger.Info().Msgf("%s received a handshake from %s",
		hs.ThisAddress.String(), hs.FromAddress.String())
	if r.routingTable == nil {
		thisId := id.NewArrayNodeID(hs.ThisAddress, hs.IdBase, hs.IdLength)
		table, err := NewTable(hs.Addresses, hs.ThisAddress, thisId)
		if err != nil {
			return nil, err
		}
		r.routingTable = table
	}
	return r.routingTable, nil
}

// NewTable constructs a routing table from the addresses of participating nodes.
// It requires the id of the node, for which the routing table is constructed,
// to calculate the common prefix of this node's id and other nodes' ids.
func NewTable(addresses []mino.Address, thisAddr mino.Address,
	thisId id.ArrayNodeID) (RoutingTable, error) {
	// random shuffle ensures that different nodes have different entries for
	// the same prefix
	randomShuffle(addresses, thisId)

	hopMap := make(map[id.StringPrefix]mino.Address)
	dela.Logger.Trace().Msgf("%s built a routing table: ", thisAddr.String())
	for _, address := range addresses {
		otherId := id.NewArrayNodeID(address, thisId.Base(), thisId.Length())
		if thisId.Equal(otherId) {
			continue
		}
		prefix, err := otherId.PrefixUntilFirstDifferentDigit(thisId)
		if err != nil {
			// TODO: can only happen if thisId == otherId, check
			continue
		}
		if _, contains := hopMap[prefix]; !contains {
			hopMap[prefix] = address
			dela.Logger.Trace().Msgf("%s -> %s (%s)", prefix.Digits,
				address.String(),
				// converting an id to prefix
				otherId.CommonPrefix(otherId).Digits)
		}
	}

	return RoutingTable{thisAddr, thisId, hopMap,
		addresses}, nil
}

func randomShuffle(addresses []mino.Address, thisId id.ArrayNodeID) {
	b, _ := id.BaseAndLenFromPlayers(len(addresses))
	base := int64(b)
	rand.Seed(int64(thisId.GetDigit(0)) +
		int64(thisId.GetDigit(1))*base +
		int64(thisId.GetDigit(2))*base*base)
	rand.Shuffle(len(addresses), func(i, j int) {
		addresses[i], addresses[j] = addresses[j], addresses[i]
	})
}

// Make implements router.RoutingTable. It creates a packet with the source
func (t RoutingTable) Make(src mino.Address, to []mino.Address,
	msg []byte) router.Packet {
	return types.NewPacket(src, msg, to...)
}

// PrepareHandshakeFor implements router.RoutingTable. It creates a handshake
// that should be sent to the distant peer when opening a relay to it.
// The peer will then generate its own routing table based on the handshake.
func (t RoutingTable) PrepareHandshakeFor(to mino.Address) router.Handshake {
	base, length := id.BaseAndLenFromPlayers(len(t.Players))
	return handshake.Handshake{IdBase: base, IdLength: length,
		FromAddress: t.thisNode, ThisAddress: to, Addresses: t.Players}
}

// Forward implements router.RoutingTable. It splits the packet into multiple,
// based on the calculated next hops.
func (t RoutingTable) Forward(packet router.Packet) (router.Routes,
	router.Voids) {
	dela.Logger.Trace().Msgf("%s: routing a message from %s to %s",
		t.thisNode.String(), packet.GetSource().String(),
		packet.GetDestination())
	routes := make(router.Routes)
	voids := make(router.Voids)

	for _, dest := range packet.GetDestination() {
		// TODO: can GetRoute ever return an error? What case that would be?
		gateway := t.GetRoute(dest)
		p, ok := routes[gateway]
		// A packet for this next hop hasn't been created yet,
		// create it and add to routes
		if !ok {
			p = types.NewPacket(packet.GetSource(), packet.GetMessage())
			routes[gateway] = p
		}

		p.(*types.Packet).Add(dest)
	}

	return routes, voids

}

// GetRoute implements router.RoutingTable. It calculates the next hop for a
// given destination.
func (t RoutingTable) GetRoute(to mino.Address) mino.Address {
	// TODO: check to != this
	toId := id.NewArrayNodeID(to, t.thisId.Base(), t.thisId.Length())
	// Take the common prefix of this node and destination + first differing
	// digit of the destination
	routingPrefix, _ := toId.PrefixUntilFirstDifferentDigit(t.thisId)
	dest, ok := t.NextHop[routingPrefix]
	if !ok {
		// TODO: compute route
	}
	dela.Logger.Info().Msgf("%s: next hop for message to %s is %s",
		t.thisNode.String(), to.String(), dest)
	return dest
}

// OnFailure implements router.RoutingTable. It changes the next hop for a
// given destination because the provided next hop is not available.
func (t RoutingTable) OnFailure(to mino.Address) error {
	// TODO: keep redundancy in the routing table, use the alternative hop
	// and mark this node as unreachable
	dela.Logger.Info().Msgf("%s failed to route the message to %s",
		t.thisNode.String(), to.String())
	return nil
}
