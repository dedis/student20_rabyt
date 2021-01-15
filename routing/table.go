package routing

import (
	"errors"
	"fmt"
	"github.com/dedis/student20_rabyt/id"
	"github.com/dedis/student20_rabyt/routing/handshake"
	"go.dedis.ch/dela"
	"go.dedis.ch/dela/mino"
	"go.dedis.ch/dela/mino/router"
	"go.dedis.ch/dela/mino/router/tree/types"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const defaultLeafSetSize = 5

// RoutingTable is a prefix-based routing implementation of router.RoutingTable.
// Each entry in the NextHop maps a prefix of this node's id (thisNode) plus
// a differing next digit to the address, where a packet whose destination has
// this prefix should be routed.
// Players contains the addresses of other nodes that this node is aware of and
// is used to select an alternative next hop when the connection to the one
// specified in NextHop fails.
type RoutingTable struct {
	thisNode    id.NodeID
	thisAddress mino.Address
	NextHop     map[id.Prefix]mino.Address
	// map from the prefix representation of address to the address.
	// Used instead of a set
	LeafSet map[id.Prefix]mino.Address
	// An internal sorted array of closest addresses
	leafSetSorted []mino.Address
	// map from the prefix representation of address to the address
	FailedHops map[id.Prefix]mino.Address
	Players    []mino.Address

	nextHopLock    sync.RWMutex
	leafSetLock    sync.RWMutex
	failedHopsLock sync.RWMutex
}

// Router implements router.Router
type Router struct {
	packetFac    router.PacketFactory
	hsFac        router.HandshakeFactory
	routingTable router.RoutingTable
}

const defaultNumHops = 5

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

// New implements router.Router. It determines the base and length of node
// ids based on the number of players and creates the routing table for the node
// that is booting the protocol.
func (r *Router) New(players mino.Players, thisAddress mino.Address) (
	router.RoutingTable, error) {
	addrs := make([]mino.Address, 0, players.Len())
	iter := players.AddressIterator()
	includedThis := false
	for iter.HasNext() {
		currentAddr := iter.GetNext()
		if currentAddr.Equal(thisAddress) {
			includedThis = true
		}
		addrs = append(addrs, currentAddr)
	}
	if !includedThis {
		addrs = append(addrs, thisAddress)
	}

	base, length := id.BaseAndLenFromPlayers(addrs, defaultNumHops)
	table, err := NewTable(addrs, id.NewArrayNodeID(thisAddress, base,
		length), thisAddress)
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
	if r.routingTable == nil {
		thisId := id.NewArrayNodeID(hs.ThisAddress, hs.IdBase, hs.IdLength)
		table, err := NewTable(hs.Addresses, thisId, hs.ThisAddress)
		if err != nil {
			return nil, err
		}
		r.routingTable = table
	}
	return r.routingTable, nil
}

func printLeafSet(addrs []mino.Address, someId id.NodeID) {
	res := ""
	for _, a := range addrs {
		res += fmt.Sprintf("%s (%s), ", id.NewArrayNodeID(a, someId.Base(),
			someId.Length()).AsPrefix().Digits, a.String())
	}
	dela.Logger.Info().Msg("current leaf set: " + res)
}

func insort(thisId id.NodeID, closestAddresses []mino.Address,
	newAddress mino.Address, maxSize int) []mino.Address {
	newId := id.NewArrayNodeID(newAddress, thisId.Base(), thisId.Length())
	for i, addr := range closestAddresses {
		curId := id.NewArrayNodeID(addr, thisId.Base(), thisId.Length())
		if thisId.CloserThan(newId, curId) {
			tmp := append(closestAddresses[:i], newAddress)
			// We added an address, therefore the last (fahrthest) address
			// is not among maxSize closest
			return append(tmp, closestAddresses[i:maxSize-1]...)
		}
	}
	if len(closestAddresses) < maxSize {
		return append(closestAddresses, newAddress)
	}
	return closestAddresses
}

// NewTable constructs a routing table from the addresses of participating nodes.
// It requires the id of the node, for which the routing table is constructed,
// to calculate the common prefix of this node's id and other nodes' ids.
func NewTable(addresses []mino.Address, thisId id.NodeID,
	thisAddress mino.Address) (router.RoutingTable, error) {
	// random shuffle ensures that different nodes have different entries for
	// the same prefix
	randomShuffle(addresses)

	dela.Logger.Info().Msg("players:")
	for _, address := range addresses {
		if address.Equal(thisAddress) {
			continue
		}
		otherId := id.NewArrayNodeID(address, thisId.Base(), thisId.Length())
		dela.Logger.Info().Msgf("%s (%s)", otherId.AsPrefix().Digits, address.String())
	}

	dela.Logger.Trace().Msgf("%s (%s) built a routing table: ",
		thisAddress.String(), thisId.AsPrefix().Digits)
	hopMap := make(map[id.Prefix]mino.Address)
	closestAddrs := make([]mino.Address, 0, defaultLeafSetSize)
	for _, address := range addresses {
		if address.Equal(thisAddress) {
			continue
		}
		otherId := id.NewArrayNodeID(address, thisId.Base(), thisId.Length())
		if otherId.Equals(thisId) {
			dela.Logger.Error().Msgf("id collision: id %s for addresses %s"+
				" and %s", thisId, thisAddress.String(), address.String())
			return nil, fmt.Errorf("id collision: id %s for addresses %s"+
				" and %s", thisId, thisAddress.String(), address.String())
		}
		prefix, err := otherId.CommonPrefixAndFirstDifferentDigit(thisId)
		if err != nil {
			return nil, fmt.Errorf("error when calculating common prefix of"+
				" ids: %s", err.Error())
		}
		if _, contains := hopMap[prefix]; !contains {
			hopMap[prefix] = address
			dela.Logger.Trace().Msgf("%s -> %s (%s)", prefix.Digits,
				address.String(), otherId.AsPrefix().Digits)
		}
		closestAddrs = insort(thisId, closestAddrs, address, defaultLeafSetSize)
		dela.Logger.Info().Msg("leafset after appending:")
		printLeafSet(closestAddrs, thisId)
	}

	leafSet := make(map[id.Prefix]mino.Address)
	for _, addr := range closestAddrs {
		curId := id.NewArrayNodeID(addr, thisId.Base(), thisId.Length())
		leafSet[curId.AsPrefix()] = addr
	}

	return &RoutingTable{
		thisNode:      thisId,
		thisAddress:   thisAddress,
		NextHop:       hopMap,
		LeafSet:       leafSet,
		leafSetSorted: closestAddrs,
		FailedHops:    make(map[id.Prefix]mino.Address),
		Players:       addresses}, nil
}

func randomShuffle(addresses []mino.Address) {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(addresses), func(i, j int) {
		addresses[i], addresses[j] = addresses[j], addresses[i]
	})
}

// Make implements router.RoutingTable. It creates a packet with the source
// address, the destination addresses and the payload.
func (t *RoutingTable) Make(src mino.Address, to []mino.Address,
	msg []byte) router.Packet {
	return types.NewPacket(src, msg, to...)
}

// PrepareHandshakeFor implements router.RoutingTable. It creates a handshake
// that should be sent to the distant peer when opening a relay to it.
// The peer will then generate its own routing table based on the handshake.
func (t *RoutingTable) PrepareHandshakeFor(to mino.Address) router.Handshake {
	return handshake.Handshake{
		IdBase:      t.thisNode.Base(),
		IdLength:    t.thisNode.Length(),
		ThisAddress: to,
		Addresses:   t.Players,
	}
}

func String(addrs []mino.Address) string {
	var sb strings.Builder
	for i, addr := range addrs {
		if addr == nil {
			sb.WriteString("nil")
		} else {
			sb.WriteString(addr.String())
		}
		if i < len(addrs) - 1 {
			sb.WriteString(", ")
		}
	}
	return sb.String()
}


// Forward implements router.RoutingTable. It splits the packet into multiple,
// based on the calculated next hops.
func (t *RoutingTable) Forward(packet router.Packet) (router.Routes,
	router.Voids) {
	dela.Logger.Trace().Stringer("this", t.thisAddress).Stringer("from",
		packet.GetSource()).Str("dests", String(packet.GetDestination())).Msgf(
		"%s: routing {%s} to %s", t.thisNode.AsPrefix().Digits, packet.GetMessage(),
		packet.GetDestination())
	routes := make(router.Routes)
	voids := make(router.Voids)

	for _, dest := range packet.GetDestination() {
		gateway, err := t.GetRoute(dest)
		if err != nil {
			voids[dest] = router.Void{Error: err}
			continue
		}

		p, ok := routes[gateway]
		// A packet for this next hop hasn't been created yet,
		// create it and add to routes
		if !ok {
			p = types.NewPacket(packet.GetSource(), packet.GetMessage())
			routes[gateway] = p
		}

		p.(*types.Packet).Add(dest)
	}

	dela.Logger.Trace().Msg("Routes:")
	for nextHop, pkt := range routes {
		dela.Logger.Trace().Stringer("nextHop",
			nextHop).Str("dests", String(pkt.GetDestination())).Msgf("{%s}",
				pkt.GetMessage())
	}
	dela.Logger.Trace().Msg("Voids:")
	for dest, err := range voids {
		dela.Logger.Trace().Stringer("dest",
			dest).Msgf("Failed to route the message: %s", err)
	}
	return routes, voids

}

func (t *RoutingTable) addrToId(addr mino.Address) id.NodeID {
	return id.NewArrayNodeID(addr, t.thisNode.Base(), t.thisNode.Length())
}

// GetRoute implements router.RoutingTable. It calculates the next hop for a
// given destination.
func (t *RoutingTable) GetRoute(to mino.Address) (mino.Address, error) {
	// Client side of the orchestrator or the server side of the orchestrator
	// which is the only player
	if len(t.Players) == 1 {
		return nil, nil
	}
	toId := t.addrToId(to)
	// Since id collisions are not expected, the only way this can happen is
	// if this node is orchestrator's server side and the message is routed to
	// orchestrator's client side. The only way the message can reach it is if
	// it is routed to nil.
	if toId.Equals(t.thisNode) && !to.Equal(t.thisAddress) {
		return nil, nil
	}

	// Take the common prefix of this node and destination + first differing
	// digit of the destination
	routingPrefix, _ := toId.CommonPrefixAndFirstDifferentDigit(t.thisNode)
	t.nextHopLock.RLock()
	nextHop, ok := t.NextHop[routingPrefix]
	t.nextHopLock.RUnlock()

	t.leafSetLock.RLock()
	defer t.leafSetLock.RUnlock()
	if _, isLeaf := t.LeafSet[toId.AsPrefix()]; isLeaf {
		// Log when we used the leaf set to go to a node that is not a next hop
		if nextHop != nil && !to.Equal(nextHop) {
			dela.Logger.Info().Msg("used leaf set")
		}
		return to, nil
	}
	if !ok {
		dela.Logger.Warn().Stringer("to", to).Msgf("%s: no entry for %s",
			t.thisNode.AsPrefix().Digits, routingPrefix.Digits)
		return nil, errors.New("No route to " + to.String())
	}
	// Find an alternative next hop
	if t.isUnreachable(nextHop) {
		for _, addr := range t.Players {
			curId := t.addrToId(addr)
			if !t.isUnreachable(addr) && t.closerToDestination(curId, toId) {
				// overwrite the next hop to dest with the alternative
				t.nextHopLock.Lock()
				t.NextHop[routingPrefix] = addr
				t.nextHopLock.Unlock()
				dela.Logger.Info().Msg("found an alternate next hop")
				return addr, nil
			}
		}
		// no alternative found, delete the entry
		t.nextHopLock.Lock()
		delete(t.NextHop, routingPrefix)
		t.nextHopLock.Unlock()
		dela.Logger.Warn().Msg("did not find any alternative")
		return nil, errors.New("No route to " + to.String())
	}
	return nextHop, nil
}

func (t *RoutingTable) closerToDestination(hop id.NodeID, dest id.NodeID) bool {
	thisPrefix, _ := dest.CommonPrefix(t.thisNode)
	hopPrefix, _ := dest.CommonPrefix(hop)
	if hopPrefix.Length() == thisPrefix.Length() {
		// hop is closer to dest than this node
		return dest.CloserThan(hop, t.thisNode)
	}
	return hopPrefix.Length() > thisPrefix.Length()
}

func (t *RoutingTable) isUnreachable(addr mino.Address) bool {
	t.failedHopsLock.RLock()
	defer t.failedHopsLock.RUnlock()
	_, is := t.FailedHops[t.addrToId(addr).AsPrefix()]
	return is
}

func (t *RoutingTable) markUnreachable(addr mino.Address) {
	nodeId := t.addrToId(addr)
	t.failedHopsLock.Lock()
	t.FailedHops[nodeId.AsPrefix()] = addr
	t.failedHopsLock.Unlock()
}

func (t *RoutingTable) updateLeafSet(unavailableAddr mino.Address) {
	nodeId := t.addrToId(unavailableAddr)
	t.leafSetLock.Lock()
	defer t.leafSetLock.Unlock()

	// Already removed from leaf set
	if _, stillLeaf := t.LeafSet[nodeId.AsPrefix()]; !stillLeaf {
		return
	}

	// Simply recompute the leaf set.
	// Not so expensive since it has a small constant size
	closestAddrs := make([]mino.Address, 0, defaultLeafSetSize)
	for _, address := range t.Players {
		if address.Equal(t.thisAddress) || t.isUnreachable(address) {
			continue
		}
		closestAddrs = insort(t.thisNode, closestAddrs, address, defaultLeafSetSize)
	}
	leafSet := make(map[id.Prefix]mino.Address)
	dela.Logger.Info().Msg("Recomputed leaf set: ")
	printLeafSet(closestAddrs, t.thisNode)
	for _, addr := range closestAddrs {
		curId := t.addrToId(addr)
		leafSet[curId.AsPrefix()] = addr
	}

	t.leafSetSorted = closestAddrs
	t.LeafSet = leafSet
}

// OnFailure implements router.RoutingTable. It marks an address as unreachable
// from this node, so that it is not chosen as next hop
func (t *RoutingTable) OnFailure(nextHop mino.Address) error {
	t.markUnreachable(nextHop)
	// mark unreachable before updating the leaf set
	t.updateLeafSet(nextHop)
	return nil
}
