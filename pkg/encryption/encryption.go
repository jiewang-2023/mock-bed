package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
)

const (
	aesCBC5P = "AES/CBC/PKCS5Padding"
	aesCFB5P = "AES/CFB/PKCS5Padding"
)

var (
	defaultKey = []byte{
		113, 114, 101, 109, 45, 97, 101, 115, 45, 107, 101, 121, 45, 112, 119, 100,
		55, 88, 116, 109, 45, 97, 101, 85, 68, 90, 122, 109, 45, 97, 101, 115,
	}

	defaultIV = []byte{
		113, 114, 101, 109, 45, 97, 101, 115, 45, 105, 118, 105, 118, 45, 49, 48,
	}
)

// PKCS7Padding 填充
func pKCS7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// PKCS7UnPadding 去除填充
func pKCS7UnPadding(data []byte) []byte {
	length := len(data)
	unPadding := int(data[length-1])
	return data[:(length - unPadding)]
}

func Encrypt(content []byte) ([]byte, error) {
	return encrypt2(aesCBC5P, content, defaultKey, defaultIV)
}

// Encrypt AES加密
func encrypt2(mode string, content, key, iv []byte) ([]byte, error) {
	if content == nil || key == nil || iv == nil {
		return nil, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// 填充
	blockSize := block.BlockSize()
	content = pKCS7Padding(content, blockSize)

	var crypted []byte
	switch mode {
	case aesCBC5P:
		blockMode := cipher.NewCBCEncrypter(block, iv)
		crypted = make([]byte, len(content))
		blockMode.CryptBlocks(crypted, content)
	case aesCFB5P:
		stream := cipher.NewCFBEncrypter(block, iv)
		crypted = make([]byte, len(content))
		stream.XORKeyStream(crypted, content)
	}

	return crypted, nil
}
func Decrypt(content []byte) ([]byte, error) {
	return decrypt2(aesCBC5P, content, defaultKey, defaultIV)
}

// Decrypt AES解密
func decrypt2(mode string, content, key, iv []byte) ([]byte, error) {
	if content == nil || key == nil || iv == nil {
		return nil, nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	var origData []byte
	switch mode {
	case aesCBC5P:
		blockMode := cipher.NewCBCDecrypter(block, iv)
		origData = make([]byte, len(content))
		blockMode.CryptBlocks(origData, content)
	case aesCFB5P:
		stream := cipher.NewCFBDecrypter(block, iv)
		origData = make([]byte, len(content))
		stream.XORKeyStream(origData, content)
	}

	// 去除填充
	return pKCS7UnPadding(origData), nil
}
