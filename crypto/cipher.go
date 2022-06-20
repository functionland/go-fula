package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
)

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
		return Cipher{}, fmt.Errorf(`somthing goes worng with random generator`)
	}
	iv, err := RandomKey(16)
	if err != nil {
		return Cipher{}, fmt.Errorf(`somthing goes worng with random generator`)
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
		err := fmt.Errorf(`plainText: "%s" has the wrong block size`, unencrypted)
		return nil, err
	}

	block, err := aes.NewCipher(c.SymKey)
	if err != nil {
		return nil, err
	}
	fmt.Println("when change", len(unencrypted))
	encData := make([]byte, len(unencrypted))

	mode := cipher.NewCBCEncrypter(block, c.Iv)
	mode.CryptBlocks(encData, unencrypted)
	fmt.Println("enc len", len(unencrypted))
	return encData, nil
}

// Decrypt decrypts cipher text string into plain text string
func (c *Cipher) Decrypt(encrypted []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.SymKey)
	if err != nil {
		return nil, err
	}
	decData := make([]byte, len(encrypted))
	fmt.Println("size of encrypted input", len(encrypted))
	mode := cipher.NewCBCDecrypter(block, c.Iv)
	mode.CryptBlocks(decData, encrypted)
	fmt.Println("size of dencrypted input", len(decData))
	// unpadaed, _ := pkcs7.Unpad(decData, aes.BlockSize)
	// fmt.Println("size of unpaded input", len(unpadaed))
	if err != nil {
		return nil, err
	}
	return decData, nil
}
