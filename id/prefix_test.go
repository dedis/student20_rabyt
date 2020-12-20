package id

import (
	"github.com/stretchr/testify/require"
	"go.dedis.ch/dela/mino/minogrpc/session"
	"strconv"
	"testing"
)

// Choose a small base to get several non-empty common prefixes
var base byte = 2
var length = 20

func TestArrayNodeID_CommonPrefix(t *testing.T) {
	ids := []ArrayNodeID{}
	for i := 0; i < 10; i++ {
		ids = append(ids, NewArrayNodeID(
			session.NewAddress(strconv.Itoa(i)),
			base, length))
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			commonPrefix, err := ids[i].CommonPrefix(ids[j])
			if err != nil {
				require.Fail(t, err.Error())
			}
			samePrefix, err := ids[j].CommonPrefix(ids[i])
			if err != nil {
				require.Fail(t, err.Error())
			}
			// a.CommonPrefix(b) == b.commonPrefix(a)
			require.Equal(t, commonPrefix, samePrefix)

			require.True(t, commonPrefix.IsPrefixOf(ids[i]))
			require.True(t, commonPrefix.IsPrefixOf(ids[j]))

			// CommonPrefix should calculate the largest common prefix
			if commonPrefix.Length() != ids[i].Length() {
				require.NotEqual(t,
					ids[i].GetDigit(commonPrefix.Length()),
					ids[j].GetDigit(commonPrefix.Length()),
					"i = %d, j = %d", i, j)
			}

			stringPrefix := commonPrefix.(StringPrefix)
			for idx := 0; idx < stringPrefix.Length(); idx++ {
				require.Equal(t, stringPrefix.Digits[idx], ids[i].GetDigit(idx))
				require.Equal(t, stringPrefix.Digits[idx], ids[j].GetDigit(idx))
			}
		}
	}
}

func TestArrayNodeID_CommonPrefixAndFirstDifferentDigit(t *testing.T) {
	ids := []ArrayNodeID{}
	for i := 0; i < 10; i++ {
		ids = append(ids, NewArrayNodeID(
			session.NewAddress(strconv.Itoa(i)),
			base, length))
	}
	for i := 0; i < len(ids); i++ {
		for j := 0; j < len(ids); j++ {
			if i == j {
				prefix, err := ids[i].CommonPrefixAndFirstDifferentDigit(ids[j])
				require.Error(t, err)
				require.Nil(t, prefix)
			} else {
				prefix, err := ids[i].CommonPrefixAndFirstDifferentDigit(ids[j])
				if err != nil {
					require.Fail(t, err.Error())
				}
				require.NotNil(t, prefix)

				require.True(t, prefix.IsPrefixOf(ids[i]))
				require.False(t, prefix.IsPrefixOf(ids[j]))

				stringPrefix := prefix.(StringPrefix)
				differentDigitIdx := stringPrefix.Length() - 1
				for idx := 0; idx < differentDigitIdx; idx++ {
					require.Equal(t, stringPrefix.Digits[idx], ids[i].GetDigit(idx))
					require.Equal(t, stringPrefix.Digits[idx], ids[j].GetDigit(idx))
				}
				require.Equal(t, stringPrefix.Digits[differentDigitIdx],
					ids[i].GetDigit(differentDigitIdx))
			}
		}
	}

}
