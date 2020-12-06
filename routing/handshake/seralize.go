package handshake

import (
	"go.dedis.ch/dela/mino"
	"go.dedis.ch/dela/mino/router/tree/types"
	"go.dedis.ch/dela/serde"
	"golang.org/x/xerrors"
)

func init() {
	RegisterHandshakeFormat(serde.FormatJSON, hsFormat{})
}

// HandshakeJSON is the JSON message for the handshake.
type HandshakeJSON struct {
	IdBase      byte
	IdLength    int
	ThisAddress []byte
	Addresses   [][]byte
}

// HandshakeFormat is the format engine to encode and decode handshake messages.
// Implements serde.FormatEngine
type hsFormat struct{}

// Encode implements serde.FormatEngine. It returns the serialized data for the
// handshake if the serialization is successful, otherwise returns an error.
func (hsFormat) Encode(ctx serde.Context, msg serde.Message) ([]byte, error) {
	hs, ok := msg.(Handshake)
	if !ok {
		return nil, xerrors.Errorf("unsupported message '%T'", msg)
	}

	addrs := make([][]byte, len(hs.Addresses))
	for i, addr := range hs.Addresses {
		raw, err := addr.MarshalText()
		if err != nil {
			return nil, xerrors.Errorf("failed to marshal address: %v", err)
		}

		addrs[i] = raw
	}
	thisAddr, err := hs.ThisAddress.MarshalText()
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal address: %v", err)
	}

	m := HandshakeJSON{
		IdBase:      hs.IdBase,
		IdLength:    hs.IdLength,
		ThisAddress: thisAddr,
		Addresses:   addrs,
	}

	data, err := ctx.Marshal(m)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal handshake: %v", err)
	}

	return data, nil
}

// Decode implements serde.FormatEngine. It populates the handshake wtih the
// information from data if data has the correct format, otherwise
// returns an error.
func (hsFormat) Decode(ctx serde.Context, data []byte) (serde.Message, error) {
	m := HandshakeJSON{}
	err := ctx.Unmarshal(data, &m)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal: %v", err)
	}

	fac := ctx.GetFactory(types.AddrKey{})

	factory, ok := fac.(mino.AddressFactory)
	if !ok {
		return nil, xerrors.Errorf("invalid address factory '%T'", fac)
	}

	addrs := make([]mino.Address, len(m.Addresses))
	for i, raw := range m.Addresses {
		addrs[i] = factory.FromText(raw)
	}

	return NewHandshake(m.IdBase, m.IdLength, factory.FromText(m.ThisAddress), addrs), nil
}
