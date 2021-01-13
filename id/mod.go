package id

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"go.dedis.ch/dela/mino"
	"math"
	"math/big"
)

type NodeID interface {
	Length() int
	Base() byte
	GetDigit(pos int) byte
	Equals(other NodeID) bool
	AsBigInt() *big.Int
	AsPrefix() StringPrefix
	CommonPrefix(other NodeID) (StringPrefix, error)
	CommonPrefixAndFirstDifferentDigit(other NodeID) (StringPrefix, error)
}

// by birthday paradox, collision probability ≈ e ^ (-n^2 / (base^length)),
// therefore base = (n ^ 2 / log probability) ^ (1 / length)
func getBase(numIds int, idLength int, collisionProb float64) int {
	return int(math.Ceil(
		math.Pow(
			-math.Log(collisionProb)/
				math.Pow(float64(numIds), 2),
			1.0/float64(idLength))))
}

func hasIdCollisions(addresses []mino.Address, idBase byte, idLength int) bool {
	addrSet := make(map[string]bool)
	idSet := make(map[Prefix]bool)
	for _, addr := range addresses {
		marshalled, err := addr.MarshalText()
		// Do not recognize an address with the same hostname as different
		// addresses based on role, which is encoded in the first byte
		// See a complete explanation in comments to hash()
		addrText := string(marshalled[1:])
		if err != nil {
			continue
		}
		_, duplicateAddr := addrSet[addrText]
		if duplicateAddr {
			continue
		}
		addrSet[addrText] = true

		id := NewArrayNodeID(addr, idBase, idLength).AsPrefix()
		_, duplicateId := idSet[id]
		if duplicateId {
			return true
		}
		idSet[id] = true
	}
	return false
}

func BaseAndLenFromPlayers(addrs []mino.Address, hopsHint int) (byte, int) {
	idLength := hopsHint
	startingCollisionProbability := 0.2
	base := getBase(len(addrs), idLength, startingCollisionProbability)
	// increase id length if base does not fit in byte
	for ; base > math.MaxUint8; idLength++ {
		base = getBase(len(addrs), idLength, startingCollisionProbability)
	}
	// increase id base until there are no id collisions
	for ; hasIdCollisions(addrs, byte(base), idLength); {
		if base == math.MaxUint8 {
			idLength += 1
			base = getBase(len(addrs), idLength, startingCollisionProbability)
		} else {
			base++
		}
	}
	return byte(base), idLength
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

func (id ArrayNodeID) AsPrefix() StringPrefix {
	prefix, _ := id.CommonPrefix(id)
	return prefix
}

const (
	OffsetA = 65
)

// CommonPrefix calculates a common prefix of two ids. Returns an error if
// bases or lengths of the ids are different.
func (id ArrayNodeID) CommonPrefix(other NodeID) (StringPrefix, error) {
	if id.Base() != other.Base() {
		return StringPrefix{}, errors.New("can't compare ids of different bases")
	}
	if id.Length() != other.Length() {
		return StringPrefix{}, errors.New("can't compare ids of different lengths")
	}
	prefix := []byte{}
	for i := 0; i < id.Length(); i++ {
		if id.GetDigit(i) != other.GetDigit(i) {
			break
		}
		prefix = append(prefix, id.GetDigit(i) + OffsetA)
	}
	return StringPrefix{string(prefix), id.Base(), OffsetA}, nil
}

// CommonPrefixAndFirstDifferentDigit returns the common prefix of the two ids
// and the first digit of this id that is different from corresponding other's
// digit. Returns an error if bases or lengths of the ids are different, or if
// ids are equal.
func (id ArrayNodeID) CommonPrefixAndFirstDifferentDigit(other NodeID) (StringPrefix,
	error) {
	commonPrefix, err := id.CommonPrefix(other)
	if err != nil {
		return StringPrefix{}, err
	}
	if commonPrefix.Length() == id.Length() {
		return StringPrefix{}, errors.New("ids are equal")
	}
	return commonPrefix.Append(id.GetDigit(commonPrefix.Length())), nil
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
	for _, value := range bytes {
		bigValue := big.NewInt(int64(value))
		bigInt.Add(bigInt, bigValue.Mul(totalPower, bigValue))
		totalPower.Mul(totalPower, power)
	}
	return bigInt
}
