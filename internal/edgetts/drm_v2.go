package edgetts

// Token Sec-MS-GEC v2.
//
// A Microsoft passou a exigir o token v2 a partir de 10/2025: o esquema antigo
// (SHA-256 em hexadecimal) devolve 403 no handshake do websocket. O v2 tem o
// formato "<CS64 16 hex>-<SHA256 customizado 64 hex>" sobre o mesmo material
// (ticks do Windows alinhados em 5 minutos + TrustedClientToken).
//
// Implementacao equivalente a referencia upstream do edge-tts, verificada
// digito a digito contra o gerador oficial.

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

const (
	winEpoch = 11644473600 // Windows epoch (1601-01-01 00:00:00 UTC)
	sToNs    = 1e9
)

const u32Mask uint64 = 0xFFFFFFFF

// SHA-256 initial vector (FIPS 180-4).
var sha256Initial = [8]uint32{
	0x6A09E667, 0xBB67AE85, 0x3C6EF372, 0xA54FF53A,
	0x510E527F, 0x9B05688C, 0x1F83D9AB, 0x5BE0CD19,
}

// Custom initial state (reversed from the browser side DRM).
var sha256StateInit = [8]uint32{
	0xFD9FBEA2, 0x91243CE7, 0xBE499094, 0xCD2E1E0A,
	0x6C006F49, 0x04BD35AB, 0xFEB0A7E4, 0x03EFFBD2,
}

var sha256Schedule = [14]uint32{
	0xEB24BF9E, 0x1558691F, 0xA0C09EE2, 0x0CFA32BA,
	0xA622EE8C, 0xD1DD9491, 0xC0DD25A0, 0xF9E70AB6,
	0xF0C31573, 0x7058CE4E, 0x7188E606, 0x00E5AB33,
	0xD258B060, 0x72766564,
}

// SHA-256 round constants.
var sha256RoundConstants = [64]uint32{
	0x428A2F98, 0x71374491, 0xB5C0FBCF, 0xE9B5DBA5, 0x3956C25B, 0x59F111F1, 0x923F82A4, 0xAB1C5ED5,
	0xD807AA98, 0x12835B01, 0x243185BE, 0x550C7DC3, 0x72BE5D74, 0x80DEB1FE, 0x9BDC06A7, 0xC19BF174,
	0xE49B69C1, 0xEFBE4786, 0x0FC19DC6, 0x240CA1CC, 0x2DE92C6F, 0x4A7484AA, 0x5CB0A9DC, 0x76F988DA,
	0x983E5152, 0xA831C66D, 0xB00327C8, 0xBF597FC7, 0xC6E00BF3, 0xD5A79147, 0x06CA6351, 0x14292967,
	0x27B70A85, 0x2E1B2138, 0x4D2C6DFC, 0x53380D13, 0x650A7354, 0x766A0ABB, 0x81C2C92E, 0x92722C85,
	0xA2BFE8A1, 0xA81A664B, 0xC24B8B70, 0xC76C51A3, 0xD192E819, 0xD6990624, 0xF40E3585, 0x106AA070,
	0x19A4C116, 0x1E376C08, 0x2748774C, 0x34B0BCB5, 0x391C0CB3, 0x4ED8AA4A, 0x5B9CCA4F, 0x682E6FF3,
	0x748F82EE, 0x78A5636F, 0x84C87814, 0x8CC70208, 0x90BEFFFA, 0xA4506CEB, 0xBEF9A3F7, 0xC67178F2,
}

const (
	sha256Adjustment   uint32 = 0xC9DC06A7
	sha256Sigma0Adjust uint32 = 0x059C0B80
	sha256Sigma1Adjust uint32 = 0x000B92C0
)

// CS64 digest constants (high term coefficient / low term offset).
var cs64Coefficients = [14][2]uint32{
	{0xE82E91B1, 0xB32700D0}, {0x44DE24C6, 0x88DFF51D},
	{0x3BEB65BC, 0x29514038}, {0xC89C5933, 0xB328CBDF},
	{0x8D1EC211, 0xD405B4D5}, {0x314EF5D5, 0x26E55F3C},
	{0xE92491B1, 0x607801C6}, {0xF0A5B9D5, 0x82B2C1FB},
	{0x06064CF3, 0xFA51A3DA}, {0xDA79FA60, 0x207CC464},
	{0xBED20560, 0x3C3C74F3}, {0x89F9C9F8, 0xD2D14B61},
	{0x35FA38C6, 0xBB04A2C1}, {0xCB49E3C6, 0xC5626167},
}

const (
	cs64Factor      uint32 = 0x82893933
	cs64Adjustment0 uint32 = 0xF2F90B1A
	cs64Adjustment1 uint32 = 0x0B1A0000
)

func rol16(v uint32) uint32 { return v<<16 | v>>16 }

func sigma0(v uint32) uint32 { return v>>7 ^ v<<25 ^ v>>18 ^ v<<14 ^ v>>3 }
func sigma1(v uint32) uint32 { return v>>17 ^ v<<15 ^ v>>19 ^ v<<13 ^ v>>10 }

// wordsLE4Even splits the data in little endian 32 bit words, zero padded and
// with an even number of words.
func wordsLE4Even(data []byte) []uint32 {
	pad := (4 - len(data)%4) % 4
	padded := make([]byte, 0, len(data)+pad)
	padded = append(padded, data...)
	padded = append(padded, make([]byte, pad)...)
	words := make([]uint32, 0, len(padded)/4+1)
	for i := 0; i+4 <= len(padded); i += 4 {
		words = append(words, binary.LittleEndian.Uint32(padded[i:]))
	}
	if len(words)%2 == 1 {
		words = append(words, 0)
	}
	return words
}

func expandSHA256Schedule(words []uint32, firstWord int) {
	for i := firstWord; i < 64; i++ {
		x, y := words[i-2], words[i-15]
		words[i] = sigma1(x) + words[i-7] + sigma0(y) + words[i-16]
	}
}

// sha256Rounds runs the SHA-256 rounds. firstRound=14 enables the custom block
// constants used by the v2 DRM variant. Intermediates use uint64 to avoid the
// wrap around of uint32 before the final mask, matching the reference.
func sha256Rounds(state [8]uint32, words []uint32, firstRound int) [8]uint32 {
	a, b, c, d, e, f, g, h := state[0], state[1], state[2], state[3], state[4], state[5], state[6], state[7]
	useBlockConstants := firstRound == 14
	for i := firstRound; i < 64; i++ {
		k := sha256RoundConstants[i]
		if useBlockConstants && i == 14 {
			k = sha256Adjustment
		}
		rotE := uint64(e)<<32 | uint64(e)
		rotA := uint64(a)<<32 | uint64(a)
		t1 := uint64(h) + ((rotE>>6)^(rotE>>11)^(rotE>>25)) + uint64(g^(e&(f^g))) + uint64(k) + uint64(words[i])
		t2 := ((rotA>>2)^(rotA>>13)^(rotA>>22)) + uint64((a&b)|(c&(a|b)))
		newA := uint32((t1 + t2) & u32Mask)
		newE := uint32((uint64(d) + t1) & u32Mask)
		a, b, c, d, e, f, g, h = newA, a, b, c, newE, e, f, g
	}
	return [8]uint32{a, b, c, d, e, f, g, h}
}

// sha256InitState computes the custom SHA-256 initial state from the first 7
// bytes (the token salt) of the v2 DRM variant.
func sha256InitState(data []byte) [8]uint32 {
	words := make([]uint32, 64)
	words[14] = uint32(data[0])<<16 | uint32(data[1])<<8 | uint32(data[2])
	words[15] = binary.BigEndian.Uint32(data[3:7])
	for i := 16; i < 30; i++ {
		value := uint64(sha256Schedule[i-16])
		if i-2 >= 14 {
			if i == 16 {
				value += uint64(sigma1(words[i-2]) ^ sha256Sigma1Adjust)
			} else {
				value += uint64(sigma1(words[i-2]))
			}
		}
		if i-7 >= 14 {
			value += uint64(words[i-7])
		}
		if i-15 >= 14 {
			if i == 29 {
				value += uint64(sigma0(words[i-15]) ^ sha256Sigma0Adjust)
			} else {
				value += uint64(sigma0(words[i-15]))
			}
		}
		words[i] = uint32(value)
	}
	words[30] = sigma1(words[28]) + words[23] + sigma0(words[15]) + words[14] + sha256Adjustment - sha256RoundConstants[14]
	expandSHA256Schedule(words, 31)
	working := sha256Rounds(sha256StateInit, words, 14)
	var out [8]uint32
	for i := 0; i < 8; i++ {
		out[i] = sha256Initial[i] + working[i]
	}
	return out
}

// sha256DigestV2 computes the custom SHA-256 digest of the token material.
func sha256DigestV2(data []byte) []byte {
	if len(data) < 7 {
		panic("token salt must contain at least seven bytes")
	}
	length := 57 + len(data)
	padLen := ((55 - length) % 64 + 64) % 64
	padded := make([]byte, 0, len(data)+padLen+16)
	padded = append(padded, data[7:]...)
	padded = append(padded, 0x80)
	padded = append(padded, make([]byte, padLen)...)
	var lenBuf [8]byte
	binary.BigEndian.PutUint64(lenBuf[:], uint64(length)*8)
	padded = append(padded, lenBuf[:]...)

	state := sha256InitState(data[:7])
	for off := 0; off+64 <= len(padded); off += 64 {
		words := make([]uint32, 64)
		for j := 0; j < 16; j++ {
			words[j] = binary.BigEndian.Uint32(padded[off+j*4:])
		}
		expandSHA256Schedule(words, 16)
		working := sha256Rounds(state, words, 0)
		for j := 0; j < 8; j++ {
			state[j] += working[j]
		}
	}
	out := make([]byte, 32)
	for i, w := range state {
		binary.BigEndian.PutUint32(out[i*4:], w)
	}
	return out
}

// cs64Digest computes the CS64 digest (two 32 bit words) of the token material.
func cs64Digest(data, digest []byte) (uint32, uint32) {
	keyA := binary.LittleEndian.Uint32(digest) | 1
	keyB := binary.LittleEndian.Uint32(digest[4:]) | 1
	factors := [2]uint32{keyA * cs64Factor, keyB * cs64Factor}

	terms := make([]uint32, 0, 64)
	offsets := make([]uint32, 0, 64)
	for i, coeff := range cs64Coefficients {
		terms = append(terms, coeff[0]*factors[i%2])
		offsets = append(offsets, coeff[1])
	}
	withPrefix := append([]byte{0}, data...)
	for i, word := range wordsLE4Even(withPrefix) {
		key, magic := keyA, uint32(0xEF0569FB)
		if i%2 == 1 {
			key, magic = keyB, 0xC31713DB
		}
		term := uint64(word) * uint64(key)
		offset := uint64(rol16(word)) * uint64(magic)
		if i == 0 {
			term += uint64(cs64Adjustment0) * uint64(factors[0])
			offset += uint64(cs64Adjustment1)
		}
		terms = append(terms, uint32(term))
		offsets = append(offsets, uint32(offset))
	}

	var swapAcc, swapLast, state, acc uint64
	for i := 0; i+1 < len(terms); i += 2 {
		x, y := terms[i], terms[i+1]
		t := rol16(offsets[i] + x)
		q := t * 0x76F50F34
		swapAcc += uint64(rol16(q))*0xC3EFEA97 + uint64(q)
		t = rol16(offsets[i+1] + y)
		q = t * 0x1881CE3C
		swapLast = (uint64(rol16(q))*0x35BD1EC9 + uint64(q)) & u32Mask
		swapAcc = (swapAcc + swapLast) & u32Mask

		state = (state*uint64(keyA) + uint64(x)) & u32Mask
		state = (uint64(rol16(uint32(state))) * 0xCF98B111) & u32Mask
		state = (uint64(rol16(uint32(state))) * 0x87085B9F) & u32Mask
		state = (uint64(rol16(uint32(state))) * 0x12CEB96D) & u32Mask
		state = (uint64(rol16(uint32(state))) * 0x257E1D83) & u32Mask
		acc += state
		t2 := (state*uint64(keyB) + uint64(y)) & u32Mask
		t2 = (uint64(rol16(uint32(t2))) * 0xA27416F5) & u32Mask
		t2 = (uint64(rol16(uint32(t2))) * 0xD38396FF) & u32Mask
		state = (uint64(rol16(uint32((uint64(rol16(uint32(t2)))*0x7C932B89)&u32Mask))) * 0xBFA49F69) & u32Mask
		acc = (acc + state) & u32Mask
	}
	return uint32(swapLast) ^ uint32(state), uint32(swapAcc) ^ uint32(acc)
}

// generateSecMSGECV2 generates the v2 token: "<CS64 16 hex>-<SHA256 64 hex>".
func generateSecMSGECV2(unixTimestamp float64) string {
	ticks := unixTimestamp + winEpoch
	ticks -= math.Mod(ticks, 300) // align down to the nearest 5 minutes
	ticks *= sToNs / 100          // Windows file time in 100 nanosecond units
	strToHash := fmt.Sprintf("%.0f%s", ticks, trustedClientToken)

	digest := sha256DigestV2([]byte(strToHash))
	cs64Hi, cs64Lo := cs64Digest([]byte(strToHash), digest)
	var cs64Buf [8]byte
	binary.LittleEndian.PutUint32(cs64Buf[:4], cs64Hi)
	binary.LittleEndian.PutUint32(cs64Buf[4:], cs64Lo)
	return strings.ToUpper(hex.EncodeToString(cs64Buf[:])) + "-" + strings.ToUpper(hex.EncodeToString(digest))
}
