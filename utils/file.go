package utils

import (
	"os"
	"strings"
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

func ExtractContent(file string) []string {
	data, _ := os.ReadFile(file)
	return strings.Split(string(data), "\n")
}
