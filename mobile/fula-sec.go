package mobile

import (
	"fmt"
	"io"
	"os"
	"crypto/aes"

	"github.com/functionland/go-fula/common"
	fCrypto "github.com/functionland/go-fula/crypto"
	filePL "github.com/functionland/go-fula/protocols/file"
	"github.com/mergermarket/go-pkcs7"
)

type FileRef struct {
	Iv  []byte
	Key []byte
	Id  string
}

func (f *Fula) EncryptSend(filePath string) (FileRef, error) {
	peer, err := f.getBox(filePL.Protocol)
	res := FileRef{}
	if err != nil {
		return res, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return res, err
	}
	defer file.Close()
	stream, err := f.node.NewStream(f.ctx, peer, filePL.Protocol)
	defer stream.Close()
	if err != nil {
		return res, err
	}
	encoder := fCrypto.NewEnReader(file)
	meta, err := common.FromFile(file)
	if err != nil {
		return res, err
	}
	id, err := filePL.SendFile(encoder, meta.ToMetaProto(), stream)
	if err != nil {
		return res, err
	}
	res.Id = *id
	res.Iv = encoder.EnCipher.Iv
	res.Key = encoder.EnCipher.SymKey
	return res, nil
}

func (f *Fula) ReceiveDecryptFile(ref FileRef, filePath string) error {
	peer, err := f.getBox(filePL.Protocol)
	if err != nil {
		return err
	}
	stream, err := f.node.NewStream(f.ctx, peer, filePL.Protocol)
	if err != nil {
		return err
	}
	defer stream.Close()
	fReader, err := filePL.ReceiveFile(stream, ref.Id)
	if err != nil {
		return err
	}
	deReader := fCrypto.NewDyReader(fReader, ref.Iv, ref.Key)
	buffer := make([]byte, 16*4)
	var buf2 []byte = nil
	file, err := os.Create(filePath)
	defer file.Close()
	if err != nil {
		return err
	}
	for {
		n, err := deReader.Read(buffer)
		fmt.Println("dec size of n ", n)
		if n > 0 {
			if buf2 != nil {
				file.Write(buf2)
			} else {
				buf2 = make([]byte, 16*4)
			}
			copy(buf2, buffer)

		}
		if err == io.EOF {
			fmt.Println("size of last input", len(buf2))
			// unpadaed, _ := pkcs7.Unpad(buf2[:48], aes.BlockSize)
			file.Write(buf2)
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
