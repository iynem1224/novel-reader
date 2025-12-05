package utils

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

var chapterPattern = regexp.MustCompile(`^第[0-9一二三四五六七八九十百千~-]+章(?:\s*：?\s*.*)?$`)

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
func decodeToUTF8(data []byte) (string, bool, error) {
	if len(data) == 0 {
		return "", false, nil
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
			return string(b), false, nil
		}
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		r := transform.NewReader(bytes.NewReader(data), unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder())
		b, err := io.ReadAll(r)
		if err == nil {
			return string(b), false, nil
		}
	}

	// If already valid UTF-8, return as-is
	if utf8.Valid(data) {
		return string(data), false, nil
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
			// Successfully decoded using a Chinese encoding -> set flag to true
			return string(b), true, nil
		}
	}

	// Fallback: treat as UTF-8 with possible invalid sequences
	return string(data), false, nil
}

func ExtractContent(file string) []string {
	data, _ := os.ReadFile(file)
	decoded, isChinese, _ := decodeToUTF8(data)
	// Normalize line endings: CRLF/CR -> LF
	normalized := strings.ReplaceAll(decoded, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if isChinese {
		lines := strings.Split(normalized, "\n")
		var cleaned []string
		
		for _, line := range lines {
			// 1. Clean all existing whitespace from both sides
			trimmed := strings.TrimLeft(line, " \t\u3000\u00A0")
			trimmed = strings.TrimRight(trimmed, " \t\r\n\u3000\u00A0")

			// 2. Skip empty lines
			if len(trimmed) == 0 {
				continue
			}

			// 3. Determine if this line should be indented
			//    Assume it is content (needs indent) by default
			needsIndent := true

			// Check A: Is it a Chapter Title? (Using your regex)
			if chapterPattern.MatchString(trimmed) {
				needsIndent = false
			}

			// Check B: Is it the Book Title? (Starts with 《)
			if strings.HasPrefix(trimmed, "《") {
				needsIndent = false
			}

			// Check C: Is it meta info? (Starts with "声明" or separators like "---")
			if strings.HasPrefix(trimmed, "声明") || strings.HasPrefix(trimmed, "-") {
				needsIndent = false
			}

			// 4. Append with or without indentation
			if needsIndent {
				cleaned = append(cleaned, "\u3000\u3000" + trimmed)
			} else {
				cleaned = append(cleaned, trimmed)
			}
		}
		return cleaned
	}
	return strings.Split(normalized, "\n")
}
