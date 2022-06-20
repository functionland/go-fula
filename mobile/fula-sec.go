package mobile

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"

	"github.com/functionland/go-fula/common"
	fCrypto "github.com/functionland/go-fula/crypto"
	filePL "github.com/functionland/go-fula/protocols/file"
)

type FileRef struct {
	Iv  []byte `json:"iv"`
	Key []byte `json:"key"`
	Id  string `json:"id"`
}

func (f *Fula) EncryptSend(filePath string) (string, error) {
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
	fileRef := &FileRef{
		Iv:  encoder.EnCipher.Iv,
		Key: encoder.EnCipher.SymKey,
		Id:  *id}
	log.Printf("%s\n", string(fileRef.Id))
	log.Printf("%s\n", string(fileRef.Iv))
	log.Printf("%s\n", string(fileRef.Key))
	// idB:=[]byte(*id)
	// liv := uint8(len(encoder.EnCipher.Iv))
	// lkey := uint8(len(encoder.EnCipher.SymKey))
	// lid := uint8(len(idB))
	// res = make([]byte, liv+lkey+lid)
	// res = append(res,encoder.EnCipher.Iv...)
	// res = append(res, encoder.EnCipher.SymKey...)
	// res = append(res,idB...)
	// res = append(res,byte(liv))
	// res = append(res,byte(lkey))
	// res = append(res,byte(lid))
	// res.Id = *id
	// res.Iv = encoder.EnCipher.Iv
	// res.Key = encoder.EnCipher.SymKey
	jsonByte, _ := json.Marshal(fileRef)
	sEnc := base64.StdEncoding.EncodeToString(jsonByte)
	return sEnc, nil
}

func (f *Fula) ReceiveDecryptFile(ref string, filePath string) error {
	jsonByte, err := base64.StdEncoding.DecodeString(ref)
	if err != nil {
		return err
	}
	var fileRef FileRef
	err = json.Unmarshal(jsonByte, &fileRef)
	log.Printf("%s\n", string(fileRef.Id))
	log.Printf("%s\n", string(fileRef.Iv))
	log.Printf("%s\n", string(fileRef.Key))
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
