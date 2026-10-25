package archive

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"errors"
	"io"
	"path"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const saveTextLimit = 2 << 20

var errNotASave = errors.New("not an RPG Maker save")

// decodeSave turns an RPG Maker save into indented JSON: MV writes
// LZString.compressToBase64(json), MZ a zlib stream. MZ hands pako's output to
// fs as a binary string, so on disk each byte above 0x7f is two UTF-8 bytes,
// and a plain zlib read of a real save fails "invalid distance too far back".
func decodeSave(name string, data []byte) ([]byte, error) {
	var raw []byte
	switch strings.ToLower(path.Ext(name)) {
	case ".rpgsave":
		s, ok := lzDecompressBase64(strings.TrimSpace(string(data)))
		if !ok {
			return nil, errNotASave
		}
		raw = []byte(s)
	case ".rmmzsave":
		var err error
		if raw, err = inflate(data); err != nil {
			if latin, ok := binaryString(data); ok {
				raw, err = inflate(latin)
			}
		}
		if err != nil {
			return nil, errNotASave
		}
	default:
		return nil, errNotASave
	}
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return nil, errNotASave
	}
	return out.Bytes(), nil
}

func inflate(b []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, 64<<20))
}

func binaryString(b []byte) ([]byte, bool) {
	if !utf8.Valid(b) {
		return nil, false
	}
	out := make([]byte, 0, len(b))
	for _, r := range string(b) {
		if r > 0xff {
			return nil, false
		}
		out = append(out, byte(r))
	}
	return out, true
}

const lzBase64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/="

// lzDecompressBase64 is lz-string's decompressFromBase64, over UTF-16 code
// units as the JavaScript original works.
func lzDecompressBase64(input string) (string, bool) {
	if input == "" {
		return "", false
	}
	next := func(i int) int {
		if i >= len(input) {
			return 0
		}
		return max(strings.IndexByte(lzBase64, input[i]), 0)
	}
	const reset = 32
	val, position, index := next(0), reset, 1
	readBits := func(n int) int {
		bits := 0
		for power := 1; power != 1<<n; power <<= 1 {
			b := val & position
			position >>= 1
			if position == 0 {
				position = reset
				val = next(index)
				index++
			}
			if b > 0 {
				bits |= power
			}
		}
		return bits
	}

	dict := [][]uint16{nil, nil, nil}
	enlargeIn, numBits := 4, 3
	var c []uint16
	switch readBits(2) {
	case 0:
		c = []uint16{uint16(readBits(8))}
	case 1:
		c = []uint16{uint16(readBits(16))}
	default:
		return "", false
	}
	dict = append(dict, c)
	w := c
	result := append([]uint16(nil), c...)
	for {
		if index > len(input) {
			return "", false
		}
		cc := readBits(numBits)
		switch cc {
		case 0, 1:
			width := 8
			if cc == 1 {
				width = 16
			}
			dict = append(dict, []uint16{uint16(readBits(width))})
			cc = len(dict) - 1
			enlargeIn--
		case 2:
			return string(utf16.Decode(result)), true
		}
		if enlargeIn == 0 {
			enlargeIn = 1 << numBits
			numBits++
		}
		var entry []uint16
		switch {
		case cc < len(dict) && dict[cc] != nil:
			entry = dict[cc]
		case cc == len(dict):
			entry = append(append([]uint16(nil), w...), w[0])
		default:
			return "", false
		}
		result = append(result, entry...)
		dict = append(dict, append(append([]uint16(nil), w...), entry[0]))
		enlargeIn--
		w = entry
		if enlargeIn == 0 {
			enlargeIn = 1 << numBits
			numBits++
		}
	}
}
