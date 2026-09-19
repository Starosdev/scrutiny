package utils

import "strings"

// LeftPad2Len pads value on the left and keeps the requested byte length.
func LeftPad2Len(value string, pad string, length int) string {
	padCount := 1 + ((length - len(pad)) / len(pad))
	padded := strings.Repeat(pad, padCount) + value
	return padded[len(padded)-length:]
}

// StripIndent removes tab indentation from multiline text.
func StripIndent(value string) string {
	return strings.ReplaceAll(value, "\t", "")
}
