package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("fula:crypto")

type IChipher interface {
	Encrypt()
	Decrypt()
}

type Cipher struct {
	Iv     []byte
	SymKey []byte
}

func NewEnCipher() (Cipher, error) {
	symKey, err := RandomKey(32)
	if err != nil {
		log.Error("somthing goes worng with random generator")
		return Cipher{}, err
	}
	iv, err := RandomKey(16)
	if err != nil {
		log.Error("somthing goes worng with random generator")
		return Cipher{}, err
	}
	return Cipher{Iv: iv, SymKey: symKey}, nil
}

func NewDeCipher(iv []byte, symKey []byte) (Cipher, error) {
	return Cipher{Iv: iv, SymKey: symKey}, nil
}

// RandomKey generate array of size n with random data.
func RandomKey(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Encrypt encrypts plain text string into cipher text string
func (c *Cipher) Encrypt(unencrypted []byte, n int) ([]byte, error) {

	if len(unencrypted)%aes.BlockSize != 0 {
		err := errors.New(fmt.Sprintf(`plainText: "%s" has the wrong block size`, unencrypted))
		return nil, err
	}

	block, err := aes.NewCipher(c.SymKey)
	if err != nil {
		return nil, err
	}
	log.Debug("unencrypted buff size", len(unencrypted))
	encData := make([]byte, len(unencrypted))

	mode := cipher.NewCBCEncrypter(block, c.Iv)
	mode.CryptBlocks(encData, unencrypted)
	log.Debug("encypted buff size:", len(encData))
	return encData, nil
}

// Decrypt decrypts cipher text string into plain text string
func (c *Cipher) Decrypt(encrypted []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.SymKey)
	if err != nil {
		return nil, err
	}
	decData := make([]byte, len(encrypted))
	log.Debug("size of encrypted input", len(encrypted))
	mode := cipher.NewCBCDecrypter(block, c.Iv)
	mode.CryptBlocks(decData, encrypted)
	log.Debug("size of dencrypted input", len(decData))
	if err != nil {
		return nil, err
	}
	return decData, nil
}
