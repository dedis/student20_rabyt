package routing

import (
	"crypto/sha256"
	"go.dedis.ch/dela/mino"
	"math/big"
)

type Id interface {
	GetLength() byte
	GetBase() byte
	GetDigit(pos int) byte
}

type BigIntId struct {
	Id   big.Int
	Base byte
}

func hash(addr mino.Address) (h big.Int) {
	sha := sha256.New()
	sha.Write([]byte(addr.String()))

	var power int64 = 0
	for _, value := range sha.Sum(nil) {
		h.Add(&h, big.NewInt(int64(value)*power))
		power <<= 8
	}
	return
}

type ArrayId struct {
	Id   []byte
	Base byte
}

func MakeArrayId(address mino.Address, base byte, len byte) Id {
	h := hash(address)
	bigBase := big.NewInt(int64(base))
	bigLen := big.NewInt(int64(len))
	h.Mod(&h, bigBase.Exp(bigBase, bigLen, nil))
	curDigit := big.NewInt(0)
	id := make([]byte, len)
	for i := 0; i < int(len); i++ {
		id[i] = byte(curDigit.Mod(&h, bigBase).Int64())
		h.Div(&h, bigBase)
	}
	return ArrayId{id, base}
}

func (id ArrayId) GetLength() byte {
	return byte(len(id.Id))
}

func (id ArrayId) GetBase() byte {
	return id.Base
}

func (id ArrayId) GetDigit(pos int) byte {
	return id.Id[pos]
}

type Prefix struct {
	Digits []byte
	Base   byte
}

func (prefix Prefix) isPrefix(id Id) bool {
	if prefix.Base != id.GetBase() {
		return false;
	}
	for i := 0; i < len(prefix.Digits); i++ {
		if prefix.Digits[i] != id.GetDigit(i) {
			return false
		}
	}
	return true
}
