package mobile

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"sync"

	fCrypto "github.com/functionland/go-fula/crypto"
	filePL "github.com/functionland/go-fula/protocols/file"
)

type FileRef struct {
	Iv  []byte `json:"iv"`
	Key []byte `json:"key"`
	Id  string `json:"id"`
}

func (f *Fula) EncryptSend(filePath string) (string, error) {
	log.Debug("encryptsend called")
	peer, err := f.getBox(filePL.Protocol)
	var res string = ""
	if err != nil {
		return res, err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return res, err
	}
	defer file.Close()
	stream, err := f.node.NewStream(f.ctx, peer, filePL.Protocol)
	if err != nil {
		return res, err
	}
	defer stream.Close()
	encoder := fCrypto.NewEncoder(file)
	meta, err := filePL.FromFile(filePath)
	if err != nil {
		return res, err
	}
	wg := sync.WaitGroup{}
	fileCh := make(chan []byte)
	go encoder.EncryptOnFly(fileCh, &wg)
	id, err := filePL.SendFile(fileCh, meta.ToMetaProto(), stream, &wg)
	if err != nil {
		return res, err
	}
	fileRef := &FileRef{
		Iv:  encoder.EnCipher.Iv,
		Key: encoder.EnCipher.SymKey,
		Id:  *id}
	jsonByte, _ := json.Marshal(fileRef)
	sEnc := base64.StdEncoding.EncodeToString(jsonByte)
	return sEnc, nil
}

func (f *Fula) ReceiveDecryptFile(ref string, filePath string) error {
	log.Debug("ReceiveDecryptFile called")
	jsonByte, err := base64.StdEncoding.DecodeString(ref)
	if err != nil {
		return err
	}
	var fileRef FileRef
	err = json.Unmarshal(jsonByte, &fileRef)
	if err != nil {
		return err
	}
	peer, err := f.getBox(filePL.Protocol)
	if err != nil {
		return err
	}
	stream, err := f.node.NewStream(f.ctx, peer, filePL.Protocol)
	if err != nil {
		return err
	}
	defer stream.Close()
	fReader, err := filePL.ReceiveFile(stream, fileRef.Id)
	if err != nil {
		return err
	}
	deReader := fCrypto.NewDecoder(fReader, fileRef.Iv, fileRef.Key)
	err = deReader.DycryptOnFly(filePath)
	if err != nil {
		return err
	}
	return nil
}

func (f *Fula) TestRef() {

}
