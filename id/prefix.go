package id

type Prefix interface {
	Length() int
	Append(byte) Prefix
	IsPrefixOf(NodeID) bool
}

// StringPrefix is an implementation of Prefix.
// Digits of the prefix are stored in a string, because this structure
// is used as a key in the routing table map.
type StringPrefix struct {
	Digits string
	Base   byte
}

// IsPrefixOf tests if this prefix is a prefix of given id
func (prefix StringPrefix) IsPrefixOf(id NodeID) bool {
	if prefix.Base != id.Base() {
		return false
	}
	for i := 0; i < len(prefix.Digits); i++ {
		if prefix.Digits[i] != id.GetDigit(i) {
			return false
		}
	}
	return true
}

// Length returns the length of prefix
func (prefix StringPrefix) Length() int {
	return len(prefix.Digits)
}

// Append constructs a new prefix by appending the digit to this prefix
func (prefix StringPrefix) Append(digit byte) Prefix {
	return StringPrefix{prefix.Digits + string(digit), prefix.Base}
}
