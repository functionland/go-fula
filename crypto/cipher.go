package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
)

var log = logging.Logger("fula/crypto")

const randomKeyGenError = "something goes wording with random generator: %s"

type IChipher interface {
	Encrypt()
	Decrypt()
}

type Cipher struct {
	Iv     []byte
	SymKey []byte
}

func NewEnCipher() *Cipher {
	symKey, err := RandomKey(32)
	if err != nil {
		log.Errorf(randomKeyGenError, err.Error())
		return &Cipher{}
	}
	iv, err := RandomKey(16)
	if err != nil {
		log.Errorf(randomKeyGenError, err.Error())
		return &Cipher{}
	}
	return &Cipher{Iv: iv, SymKey: symKey}
}

func NewDeCipher(iv []byte, symKey []byte) *Cipher {
	return &Cipher{
		Iv:     iv,
		SymKey: symKey,
	}
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
func (c *Cipher) Encrypt(unencrypted []byte) ([]byte, error) {

	if len(unencrypted)%aes.BlockSize != 0 {
		return nil, fmt.Errorf(`plainText: "%s" has the wrong block size`, unencrypted)
	}

	block, err := aes.NewCipher(c.SymKey)
	if err != nil {
		return nil, err
	}
	log.Debug("unencrypted buff size", len(unencrypted))
	encData := make([]byte, len(unencrypted))

	mode := cipher.NewCBCEncrypter(block, c.Iv)
	mode.CryptBlocks(encData, unencrypted)
	log.Debug("encrypted buff size:", len(encData))
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

	return decData, nil
}
