package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFlagSet_String(t *testing.T) {
	fset := make(FlagSet)
	fset["a"] = "something"
	fset["b"] = 20

	require.Equal(t, "something", fset.String("a"))
	require.Equal(t, "", fset.String("b"))
}

func TestFlagSet_StringSlice(t *testing.T) {
	fset := make(FlagSet)
	fset["a"] = []interface{}{"1", "2"}
	fset["b"] = 123

	require.Equal(t, []string{"1", "2"}, fset.StringSlice("a"))
	require.Nil(t, fset.StringSlice("b"))
}

func TestFlagSet_Duration(t *testing.T) {
	fset := make(FlagSet)
	fset["a"] = float64(1000.0)
	fset["b"] = 1000

	require.Equal(t, time.Duration(1000), fset.Duration("a"))
	require.Equal(t, time.Duration(0), fset.Duration("b"))
}

func TestFlagSet_Path(t *testing.T) {
	fset := make(FlagSet)
	fset["a"] = "/one/path"
	fset["b"] = 123

	require.Equal(t, "/one/path", fset.Path("a"))
	require.Equal(t, "", fset.Path("b"))
}

func TestFlagSet_Int(t *testing.T) {
	fset := make(FlagSet)
	fset["a"] = 20
	fset["b"] = "oops"

	require.Equal(t, 20, fset.Int("a"))
	require.Equal(t, 0, fset.Int("b"))
}
