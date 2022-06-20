package crypto

import (
	"crypto/aes"
	"fmt"
	"io"
	"os"

	"github.com/mergermarket/go-pkcs7"
)

const CHUNK_SIZE = 1024 * aes.BlockSize

type encoder struct {
	reader   io.Reader
	EnCipher Cipher
}

func NewEncoder(reader io.Reader) *encoder {
	encipher, _ := NewEnCipher()
	return &encoder{reader:reader, EnCipher: encipher}
}

func (c *encoder) EncryptOnFly(fileCh chan<- []byte) error {
	fileBuf := make([]byte, CHUNK_SIZE)
	for {
		n, err := c.reader.Read(fileBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if n > 0 {
			fmt.Println("befor n len", n)
			if n < len(fileBuf) {
				fileBuf, err = pkcs7.Pad(fileBuf[:n], aes.BlockSize)
			}
			encBuf, err := c.EnCipher.Encrypt(fileBuf, n)
			if err != nil {
				return err
			}
			fileCh <- encBuf
		}
	}
	close(fileCh)
	fmt.Println("Copy done")
	return nil
}

type decoder struct {
	reader   io.Reader
	DeCipher Cipher
}

func NewDecoder(reader io.Reader, iv []byte, symKey []byte) *decoder {
	decipher, _ := NewDeCipher(iv, symKey)
	return &decoder{reader: reader, DeCipher: decipher}
}

func (c *decoder) DycryptOnFly(filePath string) error {
	fmt.Println("Im caled")
	buffer := make([]byte, CHUNK_SIZE)
	var buf2 []byte = nil
	var n2 int
	file, err := os.Create(filePath)
	defer file.Close()
	if err != nil {
		return err
	}
	for {
		n, err := c.reader.Read(buffer)
		fmt.Println("dec size of n ", n)
		if n > 0 {
			n2 = n
			if buf2 != nil {
				dec, err := c.DeCipher.Decrypt(buf2)
				if err != nil {
					return err
				}
				file.Write(dec)

			} else {
				buf2 = make([]byte, CHUNK_SIZE)
			}
			copy(buf2, buffer)

		}
		if err == io.EOF {
			fmt.Println("size of last input", len(buf2))
			dec, err := c.DeCipher.Decrypt(buf2)
			if err != nil {
				return err
			}
			unpadaed, err := pkcs7.Unpad(dec[:n2], aes.BlockSize)
			if err != nil {
				file.Write(dec[:n2])
				break
			}
			file.Write(unpadaed)
			break

		}
		if err != nil {
			return err
		}
	}
	err = file.Sync()
	if err != nil {
		return err
	}
	return nil
}
