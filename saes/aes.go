package saes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
)

var AESKey = []byte("Arbitrary_Secret")

func PKCS7Unpadding(data []byte) []byte {
	count := byte(data[len(data)-1])
	return data[:len(data)-int(count)]
}
func PKCS7Padding(data []byte, blockSize int) []byte {
	size := blockSize - len(data)%blockSize
	padding := bytes.Repeat([]byte{byte(size)}, size)
	return append(data, padding...)
}

func AESDecrypt(b cipher.Block, crypted []byte) []byte {
	out := bytes.NewBuffer(nil)
	block := make([]byte, aes.BlockSize)
	for i := 0; i < len(crypted); i += aes.BlockSize {
		b.Decrypt(block, crypted[i:i+aes.BlockSize])
		out.Write(block)
	}
	return PKCS7Unpadding(out.Bytes())
}

func AESEncrypt(b cipher.Block, plain []byte) []byte {
	out := bytes.NewBuffer(nil)
	block := make([]byte, aes.BlockSize)
	plain = PKCS7Padding(plain, aes.BlockSize)
	for i := 0; i < len(plain); i += aes.BlockSize {
		b.Encrypt(block, plain[i:i+aes.BlockSize])
		out.Write(block)
	}
	return out.Bytes()
}
