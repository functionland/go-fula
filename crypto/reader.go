package crypto

import (
	"crypto/aes"
	"fmt"
	"io"
)

const CHUNK_SIZE = 2 * aes.BlockSize

type enReader struct {
	reader   io.Reader
	EnCipher Cipher
}

func NewEnReader(reader io.Reader) *enReader {
	encipher, _ := NewEnCipher()
	return &enReader{reader: reader, EnCipher: encipher}
}

func (c *enReader) Read(p []byte) (int, error) {
	if len(p) != CHUNK_SIZE {
		return 0, io.ErrShortBuffer
	}
	n, err := c.reader.Read(p)
	if err != nil {
		return n, err
	}
	buf := make([]byte, n)
	if err != nil {
		return n, err
	}
	fmt.Println("befor n len", n)
	buf, err = c.EnCipher.Encrypt(p, n)
	fmt.Println("after buf len", len(buf))
	if err != nil {
		return n, err
	}
	copy(p, buf)
	fmt.Println("Copy done")
	return len(buf), nil
}

type dyReader struct {
	reader   io.Reader
	DeCipher Cipher
}

func NewDyReader(reader io.Reader, iv []byte, symKey []byte) *dyReader {
	decipher, _ := NewDeCipher(iv, symKey)
	return &dyReader{reader: reader, DeCipher: decipher}
}

func (c *dyReader) Read(p []byte) (int, error) {
	fmt.Println("Im caled")
	if len(p) != 16*4 {
		return 0, io.ErrShortBuffer
	}
	n, err := c.reader.Read(p)
	buf := make([]byte, n)
	if err != nil {
		return n, err
	}
	buf, err = c.DeCipher.Decrypt(p)
	if err != nil {
		return n, err
	}
	copy(p, buf)

	return n, nil
}
