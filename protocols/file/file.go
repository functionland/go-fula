package file

import (
	"io"
	"io/ioutil"
	"sync"

	proto "github.com/golang/protobuf/proto"
	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
)

const Protocol = "fx/file/1"

var log = logging.Logger("fula:filePL")

func ReceiveFile(stream network.Stream, cid string) (io.Reader, error) {
	reqMsg := &Request{Type: &Request_Receive{Receive: &Chunk{Id: cid}}}
	header, err := proto.Marshal(reqMsg)
	if err != nil {
		log.Error("can not create request message")
		return nil, err
	}
	log.Debug("request message created")
	_, err = stream.Write(header)
	if err != nil {
		log.Error("sending Request Message failed")
		return nil, err
	}
	log.Debug("request message sent")
	stream.CloseWrite()
	return stream, nil
}

func ReceiveMeta(stream network.Stream, cid string) ([]byte, error) {
	reqMsg := &Request{
		Type: &Request_Meta{Meta: cid}}

	header, err := proto.Marshal(reqMsg)
	if err != nil {
		return nil, err
	}
	_, err = stream.Write(header)
	if err != nil {
		return nil, err
	}

	stream.CloseWrite()

	buf, err := ioutil.ReadAll(stream)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return buf, nil
}

func SendFile(fileCh <-chan []byte, filemeta Meta, stream network.Stream, wg *sync.WaitGroup) (*string, error) {

	//create header message
	reqMsg := &Request{Type: &Request_Send{Send: &filemeta}}
	header, err := proto.Marshal(reqMsg)
	if err != nil {
		return nil, err
	}
	log.Debug("request message created")
	//wtite the message to stream
	_, err = stream.Write(header)
	if err != nil {
		return nil, err
	}
	log.Debug("request message send")

	//write file channel to stream
	for res := range fileCh {
		log.Debug("file protocol channel data: ", res[:10])
		n, err := stream.Write(res)
		if err != nil {
			log.Error("cant write ro stream")
			return nil, err
		}
		log.Debugf("write %d on stream", n)
		wg.Done()
	}
	log.Debug("file sent")

	//closing wtite stream
	err = stream.CloseWrite()
	if err != nil {
		log.Error("cant close the write stream.")
	}
	log.Debug("stream closed succsesfuly")

	//reading stream for cid
	buf2, err := ioutil.ReadAll(stream)
	if err != nil {
		log.Error("cant close the write stream.")
		return nil, err
	}
	id := string(buf2)
	log.Debugf("received cid: %s", id)
	return &id, nil
}
