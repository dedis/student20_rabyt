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
	var intFromBytes int64 = 1 + 2*256 + 3*256*256 + 4*256*256*256
	require.Equal(t, byteArrayToBigInt(bytes).Int64(), intFromBytes)

	bytes = []byte{255, 255, 255}
	intFromBytes = 255 + 255*256 + 255*256*256
	require.Equal(t, byteArrayToBigInt(bytes).Int64(), intFromBytes)

	bytes = []byte{255, 0, 255}
	intFromBytes = 255 + 255*256*256
	require.Equal(t, byteArrayToBigInt(bytes).Int64(), intFromBytes)
}
