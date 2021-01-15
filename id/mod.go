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
	Equals(other NodeID) bool
	// Returns true if first id is closer to the current than the second,
	// false otherwise
	CloserThan(first NodeID, second NodeID) bool
	AsBigInt() *big.Int
	AsPrefix() Prefix
	CommonPrefix(other NodeID) (Prefix, error)
	CommonPrefixAndFirstDifferentDigit(other NodeID) (Prefix, error)
}

// TODO: calculate these parameters from the number of players
func BaseAndLenFromPlayers(numPlayers int) (byte, int) {
	return 16, 5
}

type ArrayNodeID struct {
	id   []byte
	base byte
}

// NewArrayNodeID constructs a NodeID by taking a hash of the address modulo
// (base ^ len), then presenting the id as an array of digits.
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

// Length returns the length of id
func (id ArrayNodeID) Length() int {
	return len(id.id)
}

// Base returns the base of id
func (id ArrayNodeID) Base() byte {
	return id.base
}

// GetDigit returns pos-th digit of id
func (id ArrayNodeID) GetDigit(pos int) byte {
	return id.id[pos]
}

// Equals tests if the id is equal to other
func (id ArrayNodeID) Equals(other NodeID) bool {
	if id.Base() != other.Base() || id.Length() != other.Length() {
		return false
	}
	switch otherId := other.(type) {
	case ArrayNodeID:
		return bytes.Equal(id.id, otherId.id)
	default:
		for i := 0; i < id.Length(); i++ {
			if id.GetDigit(i) != other.GetDigit(i) {
				return false
			}
		}
		return true
	}
}

func (id ArrayNodeID) AsBigInt() *big.Int {
	return byteArrayToBigInt(id.id)
}

func (id ArrayNodeID) AsPrefix() Prefix {
	prefix, _ := id.CommonPrefix(id)
	return prefix
}

// CommonPrefix calculates a common prefix of two ids. Returns an error if
// bases or lengths of the ids are different.
func (id ArrayNodeID) CommonPrefix(other NodeID) (Prefix, error) {
	if id.Base() != other.Base() {
		return nil, errors.New("can't compare ids of different bases")
	}
	if id.Length() != other.Length() {
		return nil, errors.New("can't compare ids of different lengths")
	}
	prefix := []byte{}
	for i := 0; i < id.Length(); i++ {
		if id.GetDigit(i) != other.GetDigit(i) {
			break
		}
		prefix = id.id[0:i+1]
	}
	return StringPrefix{string(prefix), id.Base()}, nil
}

// CommonPrefixAndFirstDifferentDigit returns the common prefix of the two ids
// and the first digit of this id that is different from corresponding other's
// digit. Returns an error if bases or lengths of the ids are different, or if
// ids are equal.
func (id ArrayNodeID) CommonPrefixAndFirstDifferentDigit(other NodeID) (Prefix, error) {
	commonPrefix, err := id.CommonPrefix(other)
	if err != nil {
		return nil, err
	}
	if commonPrefix.Length() == id.Length() {
		return nil, errors.New("ids are equal")
	}
	return commonPrefix.Append(id.GetDigit(commonPrefix.Length())), nil
}

// CloserThan returns true if the first id is closer to this id than the second,
// false otherwise
func (id ArrayNodeID) CloserThan(first NodeID, second NodeID) bool {
	thisInt := id.AsBigInt()
	firstInt := first.AsBigInt()
	secondInt := second.AsBigInt()
	// |this - first| < |this - second|
	return firstInt.Sub(thisInt, firstInt).CmpAbs(
		secondInt.Sub(thisInt, secondInt)) < 0
}

// hash returns a hash of addr as big integer
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

// byteArrayToBigInt converts an array of bytes [b0, b1, b2, ...]
// to a big int b0 + 256 * b1 + 256 ^ 2 * b2 + ...
func byteArrayToBigInt(bytes []byte) *big.Int {
	totalPower := big.NewInt(1)
	power := big.NewInt(256)
	bigInt := big.NewInt(0)
	for i := len(bytes) - 1; i >= 0; i-- {
		value := bytes[i]
		bigValue := big.NewInt(int64(value))
		bigInt.Add(bigInt, bigValue.Mul(totalPower, bigValue))
		totalPower.Mul(totalPower, power)
	}
	return bigInt
}
