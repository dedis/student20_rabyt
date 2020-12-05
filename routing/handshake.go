package routing

import (
	"go.dedis.ch/dela/mino"
	"go.dedis.ch/dela/mino/router"
	"go.dedis.ch/dela/mino/router/tree/types"
	"go.dedis.ch/dela/serde"
	"go.dedis.ch/dela/serde/registry"
	"golang.org/x/xerrors"
)

// Implements router.Handshake. Contains the addresses of other nodes, based on which the node, receiving the
// handshake, constructs a routing table.
type Handshake struct {
	idBase      byte
	idLength    int
	thisAddress mino.Address
	addresses   []mino.Address
}

var hsFormats = registry.NewSimpleRegistry()

func NewHandshake(idBase uint8, thisAddress mino.Address, addresses []mino.Address) Handshake {
	return Handshake{
		idBase:      idBase,
		thisAddress: thisAddress,
		addresses:   addresses,
	}
}

func (h Handshake) Serialize(ctx serde.Context) ([]byte, error) {
	format := hsFormats.Get(ctx.GetFormat())

	data, err := format.Encode(ctx, h)
	if err != nil {
		return nil, xerrors.Errorf("encode: %v", err)
	}

	return data, nil
}

// - implements router.HandshakeFactory
type HandshakeFactory struct {
	addrFac mino.AddressFactory
}

// NewHandshakeFactory creates a new factory.
func NewHandshakeFactory(addrFac mino.AddressFactory) HandshakeFactory {
	return HandshakeFactory{
		addrFac: addrFac,
	}
}

// Deserialize implements serde.Factory. It populates the handshake if
// appropriate, otherwise it returns an error.
func (fac HandshakeFactory) Deserialize(ctx serde.Context, data []byte) (serde.Message, error) {
	return fac.HandshakeOf(ctx, data)
}

// HandshakeOf implements router.HandshakeFactory. It populates the handshake if
// appropriate, otherwise it returns an error.
func (fac HandshakeFactory) HandshakeOf(ctx serde.Context, data []byte) (router.Handshake, error) {
	format := hsFormats.Get(ctx.GetFormat())

	ctx = serde.WithFactory(ctx, types.AddrKey{}, fac.addrFac)

	msg, err := format.Decode(ctx, data)
	if err != nil {
		return nil, xerrors.Errorf("decode: %v", err)
	}

	hs, ok := msg.(Handshake)
	if !ok {
		return nil, xerrors.Errorf("invalid handshake '%T'", msg)
	}

	return hs, nil
}
