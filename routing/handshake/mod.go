package handshake

import (
	"go.dedis.ch/dela/mino"
	"go.dedis.ch/dela/mino/router"
	"go.dedis.ch/dela/mino/router/tree/types"
	"go.dedis.ch/dela/serde"
	"go.dedis.ch/dela/serde/registry"
	"golang.org/x/xerrors"
)

// Handshake implements router.Handshake. The node, receiving the handshake,
// constructs a routing table based on this information (primarily Addresses).
type Handshake struct {
	IdBase      byte
	IdLength    int
	ThisAddress mino.Address
	Addresses   []mino.Address
}

var hsFormats = registry.NewSimpleRegistry()

// RegisterHandshakeFormat registers the engine for the provided format.
func RegisterHandshakeFormat(f serde.Format, e serde.FormatEngine) {
	hsFormats.Register(f, e)
}

// NewHandshake creates a handshake from the arguments
func NewHandshake(idBase uint8, idLength int, thisAddress mino.Address,
	addresses []mino.Address) Handshake {
	return Handshake{
		IdBase:      idBase,
		IdLength:    idLength,
		ThisAddress: thisAddress,
		Addresses:   addresses,
	}
}

// Serialize encodes the handshake as HandshakeJSON
func (h Handshake) Serialize(ctx serde.Context) ([]byte, error) {
	format := hsFormats.Get(ctx.GetFormat())

	data, err := format.Encode(ctx, h)
	if err != nil {
		return nil, xerrors.Errorf("encode: %v", err)
	}

	return data, nil
}

// HandshakeFactory implements router.HandshakeFactory
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
