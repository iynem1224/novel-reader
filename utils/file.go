package utils

import (
	"bytes"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func IsValidTxt(path string) bool {
	if !strings.HasSuffix(path, ".txt") {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

// decodeToUTF8 attempts to decode arbitrary text bytes to UTF-8.
// It supports:
// - UTF-8 (with or without BOM)
// - UTF-16 LE/BE with BOM
// - GB18030/GBK (common for Simplified Chinese)
func decodeToUTF8(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}

	// Handle UTF-8 BOM
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		data = data[3:]
	}

	// Handle UTF-16 BOMs
	if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
		r := transform.NewReader(bytes.NewReader(data), unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder())
		b, err := io.ReadAll(r)
		if err == nil {
			return string(b), nil
		}
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		r := transform.NewReader(bytes.NewReader(data), unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder())
		b, err := io.ReadAll(r)
		if err == nil {
			return string(b), nil
		}
	}

	// If already valid UTF-8, return as-is
	if utf8.Valid(data) {
		return string(data), nil
	}

	// Try common Simplified Chinese encodings
	for _, dec := range []transform.Transformer{
		simplifiedchinese.GB18030.NewDecoder(),
		simplifiedchinese.GBK.NewDecoder(),
		simplifiedchinese.HZGB2312.NewDecoder(),
	} {
		r := transform.NewReader(bytes.NewReader(data), dec)
		b, err := io.ReadAll(r)
		if err == nil && utf8.Valid(b) {
			return string(b), nil
		}
	}

	// Fallback: treat as UTF-8 with possible invalid sequences
	return string(data), nil
}

func ExtractContent(file string) []string {
	data, _ := os.ReadFile(file)
	decoded, _ := decodeToUTF8(data)
	// Normalize line endings: CRLF/CR -> LF
	normalized := strings.ReplaceAll(decoded, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}
