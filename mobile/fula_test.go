package mobile

import (
	"crypto/md5"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	filePL "github.com/functionland/go-fula/protocols/file"
	proto "github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/peer"
)

const BOX = "/p2p/12D3KooWJVDdxaWYxSEC3M8oK57swu1jc36YYMZihbLmiQjQ2B26"
const BOX_LOOPBACK = "/ip4/127.0.0.1/tcp/4002/p2p/12D3KooWGrkcHUBzAAuYhMRxBreCgofKKDhLgR84FbawknJZHwK1"

func TestNew(t *testing.T) {

	_, err := NewFula("./repo")
	if err != nil {
		t.Error(err)
	}
}

func TestAddBox(t *testing.T) {
	fula, err := NewFula("./repo")
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("Fail to adding peer: \n", err)
		return
	}
	want, _ := peer.AddrInfoFromString(BOX)
	peer, err := fula.getBox()
	if err != nil {
		t.Error(err)
	}
	if want.ID == peer {
		return
	}
	t.Error("Peer Was Not added")
}

func TestAddBoxLoopBack(t *testing.T) {
	fula, err := NewFula("./repo")
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("Mobile Can not accept loopback")
	}
}

func TestFileProtocol(t *testing.T) {
	fula, err := NewFula("./repo")
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("add error")
	}
	tmp := "./tmp"
	if _, err := os.Stat(tmp); os.IsNotExist(err) {
		err := os.Mkdir(tmp, 0755)
		if err != nil {
			t.Error("wired error", err)
			return
		}
	}
	t.Log("fula ready")
	files, err := ioutil.ReadDir("./test_assets")
	if err != nil {
		t.Error(err)
	}
	for _, file := range files {
		if !file.IsDir() {
			upload := "./test_assets/" + file.Name()
			cid, err := fula.Send(upload)
			if err != nil {
				t.Error("send failed", err)
				return
			}
			bytes, err := fula.ReceiveFileInfo(cid)
			t.Log("File with CID: ", cid)
			if err != nil {
				t.Error("download Failed", err)
				return
			}
			meta := &filePL.Meta{}
			err = proto.Unmarshal(bytes, meta)
			if err != nil {
				t.Error("parsing Meta failed", err)
				return
			}
			download := tmp + "/" + meta.Name
			err = fula.ReceiveFile(cid, download)
			if err != nil {
				t.Error("Receive File failed", err)
				return
			}
			if !fileDiff(upload, download) {
				t.Error("Somthing wrong! files are not equal", err)
				return
			}
			t.Logf("successfully test send and receive of %s", upload)
			time.Sleep(time.Second)
		}

	}

}

func TestEncryption(t *testing.T) {
	fula, err := NewFula("./repo")
	if err != nil {
		t.Error(err)
	}
	err = fula.AddBox(BOX)
	if err != nil {
		t.Error("can not add box", err)
	}
	tmp := "./tmp"
	if _, err := os.Stat(tmp); os.IsNotExist(err) {
		err := os.Mkdir(tmp, 0755)
		if err != nil {
			t.Error("wired error", err)
			return
		}
	}
	files, err := ioutil.ReadDir("./test_assets")
	if err != nil {
		t.Error(err)
	}
	for _, file := range files {
		if !file.IsDir() {
			upload := "./test_assets/" + file.Name()
			ref, err := fula.EncryptSend(upload)
			if err != nil {
				t.Error("send failed", err)
				return
			}
			download := tmp + "/" + ref
			err = fula.ReceiveDecryptFile(ref, download)
			if err != nil {
				t.Error("receive File failed", err)
				return
			}
			if !fileDiff(upload, download) {
				t.Error("somthing wrong! files are not equal", err)
				return
			}
		}
		time.Sleep(time.Second)
	}

}

func fileDiff(path1 string, path2 string) bool {
	hash1 := md5File(path1)
	hash2 := md5File(path2)
	return string(hash1) == string(hash2)
}

func md5File(path string) []byte {
	file, err := os.Open(path)

	if err != nil {
		panic(err)
	}

	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)

	if err != nil {
		panic(err)
	}
	return hash.Sum(nil)
}
