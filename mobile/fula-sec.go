package mobile

import (
	"os"

	"github.com/functionland/go-fula/common"
	fCrypto "github.com/functionland/go-fula/crypto"
	filePL "github.com/functionland/go-fula/protocols/file"
)

type FileRef struct {
	Iv  []byte
	Key []byte
	Id  string
}

func (f *Fula) EncryptSend(filePath string) ([]byte, error) {
	peer, err := f.getBox(filePL.Protocol)
	var res []byte = nil
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
	encoder := fCrypto.NewEncoder(file)
	meta, err := common.FromFile(file)
	if err != nil {
		return res, err
	}
	fileCh := make(chan []byte)
	go encoder.EncryptOnFly(fileCh)
	id, err := filePL.SendFile(fileCh, meta.ToMetaProto(), stream)
	if err != nil {
		return res, err
	}
	idB:=[]byte(*id)
	res = make([]byte, len(encoder.EnCipher.Iv)+len(encoder.EnCipher.SymKey)+len(idB))
	res = append(res,encoder.EnCipher.Iv...)
	res = append(res, encoder.EnCipher.SymKey...)
	res = append(res,idB...)
	// res.Id = *id
	// res.Iv = encoder.EnCipher.Iv
	// res.Key = encoder.EnCipher.SymKey
	return res, nil
}

func (f *Fula) ReceiveDecryptFile(ref FileRef, filePath string) (error) {
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
	deReader := fCrypto.NewDecoder(fReader, ref.Iv, ref.Key)
	err = deReader.DycryptOnFly(filePath)
	if err != nil {
		return err
	}
	return nil
}
