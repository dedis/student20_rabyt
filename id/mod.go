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
	CommonPrefix(other ArrayNodeID) StringPrefix
	PrefixUntilFirstDifferentDigit(other ArrayNodeID) (StringPrefix, error)
}

func BaseAndLenFromPlayers(numPlayers int) (base byte, len int) {
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

func (id ArrayNodeID) CommonPrefix(other ArrayNodeID) Prefix {
	// TODO: report an error if bases or lengths are different
	prefix := []byte{}
	var offset byte = 65
	for i := 0; i < id.Length(); i++ {
		if id.GetDigit(i) != other.GetDigit(i) {
			break
		}
		prefix = append(prefix, id.GetDigit(i) + offset)
	}
	return StringPrefix{string(prefix), id.Base(), offset}
}

func (id ArrayNodeID) PrefixUntilFirstDifferentDigit(other ArrayNodeID) (StringPrefix,
	error) {
	//if id.GetBase() != other.GetBase() {
	//	return nil, errors.New("can't compare ids of different bases")
	//}
	//if id.GetLength() != other.GetLength() {
	//	return nil, errors.New("can't compare ids of different lengths")
	//}
	commonPrefix := id.CommonPrefix(other)
	//if commonPrefix.GetLength() == id.GetLength() {
	//	return nil, errors.New("ids are equal")
	//}
	return commonPrefix.Append(id.GetDigit(commonPrefix.Length())), nil
}

func (id ArrayNodeID) Equal(other ArrayNodeID) bool {
	if id.base != other.base || id.Length() != other.Length() {
		return false;
	}
	for i := 0; i < id.Length(); i++ {
		if id.GetDigit(i) != other.GetDigit(i) {
			return false;
		}
	}
	return true;
}

// Returns a hash of addr as big integer
func hash(addr mino.Address) *big.Int {
	sha := sha256.New()
	sha.Write([]byte(addr.String()))
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
