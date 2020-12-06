package id

import (
	"crypto/sha256"
	"go.dedis.ch/dela/mino"
	"math/big"
)

type Id interface {
	GetLength() int
	GetBase() byte
	GetDigit(pos int) byte
	CommonPrefix(other ArrayId) Prefix
	PrefixUntilFirstDifferentDigit(other ArrayId) (PrefixImpl, error)
}

func BaseAndLenFromPlayers(numPlayers int) (base byte, len int) {
	return 16, 3
}

type ArrayId struct {
	Id   []byte
	Base byte
}

// Constructs an Id by taking a hash of the address modulo (base ^ len), then presenting the id as an array of digits.
func MakeArrayId(address mino.Address, base byte, len int) ArrayId {
	h := hash(address)
	bigBase := big.NewInt(int64(base))
	bigLen := big.NewInt(int64(len))
	h.Mod(h, bigBase.Exp(bigBase, bigLen, nil))
	curDigit := big.NewInt(0)
	id := make([]byte, len)
	for i := 0; i < len; i++ {
		id[i] = byte(curDigit.Mod(h, bigBase).Int64())
		h.Div(h, bigBase)
	}
	return ArrayId{id, base}
}

func (id ArrayId) GetLength() int {
	return len(id.Id)
}

func (id ArrayId) GetBase() byte {
	return id.Base
}

func (id ArrayId) GetDigit(pos int) byte {
	return id.Id[pos]
}

func (id ArrayId) CommonPrefix(other ArrayId) Prefix {
	// TODO: report an error if bases or lengths are different
	prefix := []byte{}
	for i := 0; i < id.GetLength(); i++ {
		if id.GetDigit(i) != other.GetDigit(i) {
			break
		}
		prefix = id.Id[0:i]
	}
	return PrefixImpl{string(prefix), id.Base}
}

func (id ArrayId) PrefixUntilFirstDifferentDigit(other ArrayId) (PrefixImpl, error) {
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
	return commonPrefix.Append(id.GetDigit(commonPrefix.GetLength())), nil
}

func hash(addr mino.Address) (h *big.Int) {
	sha := sha256.New()
	sha.Write([]byte(addr.String()))

	totalPower := big.NewInt(1)
	power := big.NewInt(8)
	h = big.NewInt(0)
	for _, value := range sha.Sum(nil) {
		bigValue := big.NewInt(int64(value))
		h.Add(h, bigValue.Mul(totalPower, bigValue))
		totalPower.Mul(totalPower, power)
	}
	return
}
