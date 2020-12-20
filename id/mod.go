package id

import (
	"crypto/sha256"
	"go.dedis.ch/dela/mino"
	"math/big"
)

type NodeID interface {
	Length() int
	Base() byte
	GetDigit(pos int) byte
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

// Returns a hash of addr as big integer
func hash(addr mino.Address) *big.Int {
	sha := sha256.New()
	marshalled, err := addr.MarshalText()
	if err != nil {
		marshalled = []byte(addr.String())
	}
	sha.Write(marshalled)
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
