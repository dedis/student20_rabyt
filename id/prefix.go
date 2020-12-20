package id

type Prefix interface {
	Length() int
	Append(byte) StringPrefix
	IsPrefixOf(id ArrayNodeID) bool
}

// Digits of the prefix have to be stored in a string, because this structure
// is used as a key in the routing table map.
type StringPrefix struct {
	Digits string
	Base   byte
	Offset byte
}

// Tests if this prefix is a prefix of given id
func (prefix StringPrefix) IsPrefixOf(id ArrayNodeID) bool {
	if prefix.Base != id.Base() {
		return false
	}
	for i := 0; i < len(prefix.Digits); i++ {
		if prefix.Digits[i] - prefix.Offset != id.GetDigit(i) {
			return false
		}
	}
	return true
}

// Returns the length of prefix
func (prefix StringPrefix) Length() int {
	return len(prefix.Digits)
}

// Constructs a new prefix by appending the digit to this prefix
func (prefix StringPrefix) Append(digit byte) StringPrefix {
	return StringPrefix{prefix.Digits + string(digit + prefix.Offset),
		prefix.Base, prefix.Offset}
}
