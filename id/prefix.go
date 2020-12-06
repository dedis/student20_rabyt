package id

type Prefix interface {
	GetLength() int
	isPrefixOf(id ArrayId) bool
	Append(byte) PrefixImpl
}

// Digits of the prefix have to be stored in a string, because this structure is used as a key in the routing table map.
type PrefixImpl struct {
	Digits string
	Base   byte
}

func (prefix PrefixImpl) isPrefixOf(id ArrayId) bool {
	if prefix.Base != id.GetBase() {
		return false
	}
	for i := 0; i < len(prefix.Digits); i++ {
		if prefix.Digits[i] != id.GetDigit(i) {
			return false
		}
	}
	return true
}

func (prefix PrefixImpl) GetLength() int {
	return len(prefix.Digits)
}

func (prefix PrefixImpl) Append(digit byte) PrefixImpl {
	return PrefixImpl{prefix.Digits + string(digit), prefix.Base}
}
