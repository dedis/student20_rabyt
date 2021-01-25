package id

import (
	"github.com/stretchr/testify/require"
	"go.dedis.ch/dela/mino/minogrpc/session"
	"testing"
)

var testBase byte = 10
var testLen = 5

func TestNewArrayNodeID_CorrectBaseAndLength(t *testing.T) {
	id := NewArrayNodeID(session.NewAddress("127.0.0.1:2000"), testBase, testLen)
	require.Equal(t, testBase, id.Base())
	require.Equal(t, testLen, id.Length())
	for i := 0; i < testLen; i++ {
		require.Less(t, id.GetDigit(i), testBase,
			"%d-th digit is greater or equal to base: %d", i, id.GetDigit(i))
	}
}

func TestArrayNodeID_Equals(t *testing.T) {
	host := "127.0.0.1:2000"
	firstId := NewArrayNodeID(session.NewAddress(host), testBase, testLen)
	secondId := NewArrayNodeID(session.NewAddress(host), testBase, testLen)
	require.True(t, firstId.Equals(secondId))
	unequalId := NewArrayNodeID(session.NewAddress(host+"0"), testBase, testLen)
	require.False(t, firstId.Equals(unequalId))
}

func TestArrayNodeID_FollowerEqualOrchestrator(t *testing.T) {
	follower := session.NewAddress("127.0.0.1:2000")
	followerId := NewArrayNodeID(follower, testBase, testLen)
	orchestrator := session.NewOrchestratorAddress(follower)
	orchestratorId := NewArrayNodeID(orchestrator, testBase, testLen)
	require.True(t, followerId.Equals(orchestratorId))
}

func Test_byteArrayToBigInt(t *testing.T) {
	bytes := []byte{1, 2, 3, 4}
	var intFromBytes int64 = 1*256*256*256 + 2*256*256 + 3*256 + 4
	require.Equal(t, byteArrayToBigInt(bytes).Int64(), intFromBytes)

	bytes = []byte{255, 255, 255}
	intFromBytes = 255*256*256 + 255*256 + 255
	require.Equal(t, byteArrayToBigInt(bytes).Int64(), intFromBytes)

	bytes = []byte{255, 0, 255}
	intFromBytes = 255*256*256 + 255
	require.Equal(t, byteArrayToBigInt(bytes).Int64(), intFromBytes)
}

func TestArrayNodeID_Distance(t *testing.T) {
	firstId := ArrayNodeID{
		id:   []byte{0, 0, 0},
		base: testBase,
	}
	secondId := ArrayNodeID{
		id:   []byte{testBase - 1, testBase - 1, testBase - 1},
		base: testBase,
	}
	require.Equal(t, int64(1), firstId.Distance(secondId).Int64())
	thirdId := ArrayNodeID{
		id:   []byte{1, 0, 0},
		base: testBase,
	}
	var base_i64 = int64(testBase)
	require.Equal(t, 1 * base_i64 * base_i64, firstId.Distance(thirdId).Int64())
}
