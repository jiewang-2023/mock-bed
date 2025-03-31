package main

import (
	"testing"

	"mock-bed/pkg/encryption"
)

func TestNum(t *testing.T) {
	bs := []byte{0x55, 4}
	encryptedData, _ := encryption.Encrypt(bs)

	println(len(bs))
	println(len(encryptedData))

	for i := 401; i < 800; i++ {
	}
}
