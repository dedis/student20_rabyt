package id

type Prefix interface {
	Length() int
	Append(byte) Prefix
	IsPrefixOf(NodeID) bool
}

// Digits of the prefix have to be stored in a string, because this structure
// is used as a key in the routing table map.
type StringPrefix struct {
	Digits string
	Base   byte
}

// Tests if this prefix is a prefix of given id
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

// Returns the length of prefix
func (prefix StringPrefix) Length() int {
	return len(prefix.Digits)
}

// Constructs a new prefix by appending the digit to this prefix
func (prefix StringPrefix) Append(digit byte) Prefix {
	return StringPrefix{prefix.Digits + string(digit), prefix.Base}
}
