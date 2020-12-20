package id

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"go.dedis.ch/dela/mino"
	"math/big"
)

type NodeID interface {
	Length() int
	Base() byte
	GetDigit(pos int) byte
	Equals(other ArrayNodeID) bool
	AsPrefix() StringPrefix
	CommonPrefix(other ArrayNodeID) (StringPrefix, error)
	CommonPrefixAndFirstDifferentDigit(other ArrayNodeID) (StringPrefix, error)
}

// TODO: calculate these parameters from the number of players
func BaseAndLenFromPlayers(numPlayers int) (byte, int) {
	return 16, 3
}

type ArrayNodeID struct {
	id   []byte
	base byte
}

// Constructs a NodeID by taking a hash of the address modulo (base ^ len),
// then presenting the id as an array of digits.
func NewArrayNodeID(address mino.Address, base byte, len int) ArrayNodeID {
	h := hash(address)
	bigBase := big.NewInt(int64(base))
	bigLen := big.NewInt(int64(len))
	maxId := big.NewInt(0)
	maxId = maxId.Exp(bigBase, bigLen, nil)
	h.Mod(h, maxId)
	curDigit := big.NewInt(0)
	id := make([]byte, len)
	for i := 0; i < len; i++ {
		id[i] = byte(curDigit.Mod(h, bigBase).Int64())
		h.Div(h, bigBase)
	}
	return ArrayNodeID{id, base}
}

// Returns the length of id
func (id ArrayNodeID) Length() int {
	return len(id.id)
}

// Returns the base of id
func (id ArrayNodeID) Base() byte {
	return id.base
}

// Returns pos-th digit of id
func (id ArrayNodeID) GetDigit(pos int) byte {
	return id.id[pos]
}

func (id ArrayNodeID) Equals(other ArrayNodeID) bool {
	if id.Base() != other.Base() || id.Length() != other.Length() {
		return false
	}

	return id.base == other.base && id.Length() ==
		other.Length() && bytes.Equal(id.id, other.id)
}

func (id ArrayNodeID) AsPrefix() StringPrefix {
	prefix, _ := id.CommonPrefix(id)
	return prefix
}

func (id ArrayNodeID) CommonPrefix(other ArrayNodeID) (StringPrefix, error) {
	if id.Base() != other.Base() {
		return StringPrefix{"", 0, 0},
		errors.New("can't compare ids of different bases")
	}
	if id.Length() != other.Length() {
		return StringPrefix{"", 0, 0},
		errors.New("can't compare ids of different lengths")
	}
	prefix := []byte{}
	var offset byte = 65
	for i := 0; i < id.Length(); i++ {
		if id.GetDigit(i) != other.GetDigit(i) {
			break
		}
		prefix = append(prefix, id.GetDigit(i) + offset)
	}
	return StringPrefix{string(prefix), id.Base(), offset}, nil
}

func (id ArrayNodeID) CommonPrefixAndFirstDifferentDigit(other ArrayNodeID) (StringPrefix,
	error) {
	commonPrefix, err := id.CommonPrefix(other)
	if err != nil {
		return commonPrefix, err
	}
	if commonPrefix.Length() == id.Length() {
		return commonPrefix, errors.New("ids are equal")
	}
	return commonPrefix.Append(id.GetDigit(commonPrefix.Length())), nil
}

// Returns a hash of addr as big integer
func hash(addr mino.Address) *big.Int {
	sha := sha256.New()
	marshalled, err := addr.MarshalText()
	if err != nil {
		marshalled = []byte(addr.String())
	}
	// A hack to accommodate for minogrpc's design:
	// 1) the first byte is used to indicate if a node is orchestrator or not
	// 2) the only way to reach the orchestrator is to route a message to nil
	//    from its server side, which has the same address but orchestrator byte
	//    set to f.
	// We therefore have to ignore if a node is the orchestrator to be able to
	// route the message first to its server side, then from the server side to
	// the client side.
	sha.Write(marshalled[1:])
	return byteArrayToBigInt(sha.Sum(nil))
}

// Converts an array of bytes [b0, b1, b2, ...]
// to a big int b0 + 256 * b1 + 256 ^ 2 * b2 + ...
func byteArrayToBigInt(bytes []byte) *big.Int {
	totalPower := big.NewInt(1)
	power := big.NewInt(256)
	bigInt := big.NewInt(0)
	for _, value := range bytes {
		bigValue := big.NewInt(int64(value))
		bigInt.Add(bigInt, bigValue.Mul(totalPower, bigValue))
		totalPower.Mul(totalPower, power)
	}
	return bigInt
}
