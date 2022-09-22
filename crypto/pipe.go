package crypto

import (
	"crypto/aes"
	"io"
	"os"
	"sync"

	"github.com/mergermarket/go-pkcs7"
)

const ChunkSize = 1024 * aes.BlockSize

type encoder struct {
	reader   io.Reader
	EnCipher Cipher
}

func NewEncoder(reader io.Reader) *encoder {
	encipher := NewEnCipher()
	return &encoder{reader: reader, EnCipher: *encipher}
}

func (c *encoder) EncryptOnFly(fileCh chan<- []byte, wg *sync.WaitGroup) error {
	defer close(fileCh)
	log.Debug("start EncryptOnFly")
	fileBuf := make([]byte, ChunkSize)
	for {
		n, err := c.reader.Read(fileBuf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if n > 0 {
			log.Debugf("size of buffer befor encoding: %d", n)
			if n < len(fileBuf) {
				fileBuf, err = pkcs7.Pad(fileBuf[:n], aes.BlockSize)
				if err != nil {

					return err
				}
			}
			encBuf, err := c.EnCipher.Encrypt(fileBuf)
			if err != nil {
				return err
			}
			log.Debugf("Size of buffer after encoding: %d", len(encBuf))
			wg.Add(1)
			fileCh <- encBuf
			log.Debug("data pushed to file channel")
			wg.Wait()
		}
	}

	log.Debug("EncryptOnFly finished")
	return nil
}

type decoder struct {
	reader   io.Reader
	DeCipher Cipher
}

func NewDecoder(reader io.Reader, iv []byte, symKey []byte) *decoder {
	decipher := NewDeCipher(iv, symKey)
	return &decoder{reader: reader, DeCipher: *decipher}
}

func (c *decoder) DecryptOnFly(filePath string) error {
	const size = 16
	buffer := make([]byte, size)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var (
		cache      []byte = nil
		cacheDec   []byte = nil
		chunkIndex        = 0
	)

	for {
		n, err := c.reader.Read(buffer)
		if err == io.EOF {
			if cacheDec != nil {
				file.Write(cacheDec)
				cacheDec = nil
			}
			dec, err := c.DeCipher.Decrypt(cache[:chunkIndex])
			if err != nil {
				return err
			}
			unpadaed, err := pkcs7.Unpad(dec, aes.BlockSize)
			if err != nil {
				file.Write(dec)
			} else {
				file.Write(unpadaed)
			}
			break
		}
		if n > 0 {
			cache = append(cache, buffer...)
			chunkIndex += 1 * size
			if chunkIndex >= ChunkSize {
				dec, err := c.DeCipher.Decrypt(cache)
				if err != nil {
					return err
				}
				if cacheDec != nil {
					file.Write(cacheDec)
					cacheDec = nil
				}
				cacheDec = dec
				cache = nil
				chunkIndex = 0
			}
		}

	}

	err = file.Sync()
	if err != nil {
		return err
	}
	return nil
}
