package psref

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageUnescape(t *testing.T) {
	s := unescapeImage("http%3a%2f%2fpsref.lenovo.com%2fsyspool%2fSys%2fImage%2fLegion%2fLenovo_Legion_5P_15IMH05H%2fCompressedimageForMobileShare%2fLenovo_Legion_5P_15IMH05H_CT1_01.png")
	require.Equal(t, "http://psref.lenovo.com/syspool/Sys/Image/Legion/Lenovo_Legion_5P_15IMH05H/CompressedimageForMobileShare/Lenovo_Legion_5P_15IMH05H_CT1_01.png", s)
}
