package file

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"sync"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/network"
	"google.golang.org/protobuf/proto"
)

const Protocol = "fx/file/1"

var log = logging.Logger("fula:filePL")

func encodPerfixLenth(b []byte) []byte {
	perfix := make([]byte, binary.MaxVarintLen64)
	perfixsize := binary.PutUvarint(perfix, uint64(len(b)))
	header := append(perfix[:perfixsize], b...)
	return header
}

func ReceiveFile(stream network.Stream, cid string) (io.Reader, error) {
	reqMsg := &Request{Type: &Request_Receive{Receive: &Chunk{Id: cid}}}
	header, err := proto.Marshal(reqMsg)
	if err != nil {
		log.Error("can not create request message")
		return nil, err
	}
	log.Debug("request message created")
	header = encodPerfixLenth(header)
	_, err = stream.Write(header)
	if err != nil {
		log.Error("sending Request Message failed", err)
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
		log.Error("marshaling Meta Message failed", err)
		return nil, err
	}
	header = encodPerfixLenth(header)
	n, err := stream.Write(header)
	if err != nil {
		log.Error("write header faild", err)
		return nil, err
	}

	log.Debugf("wrote meta header with size of %d on stream", n)
	err = stream.CloseWrite()
	if err != nil {
		log.Error("failed to close write sream with error: ", err)
		return nil, err
	}
	buf, err := ioutil.ReadAll(stream)
	if err != nil {
		log.Error("failed to read Meta buffer. ", err)
		return nil, err
	}
	err = stream.CloseRead()
	if err != nil {
		log.Error("failed to close read stream. ", err)
		stream.Reset()
		return nil, err
	}
	stream.Close()
	return buf, nil
}

func SendFile(fileCh <-chan []byte, filemeta *Meta, stream network.Stream, wg *sync.WaitGroup) (*string, error) {
	//create header message
	reqMsg := &Request{Type: &Request_Send{Send: filemeta}}
	msg, err := proto.Marshal(reqMsg)
	if err != nil {
		return nil, err
	}
	header:=encodPerfixLenth(msg)
	n, err := stream.Write(header)
	log.Debugf("header of size %d wrote on stream", n)
	if err != nil {
		log.Debugf("failed to wrote header on stream ", err)
		stream.Reset()
		return nil, err
	}
	log.Debug("request message send")

	//write file channel to stream
	for res := range fileCh {
		log.Debug("file protocol channel data: ", res[:10])
		n, err := stream.Write(res)
		if err != nil {
			log.Error("can not write on stream ", err)
			stream.Reset()
			return nil, err
		}
		log.Debugf("write %d on stream", n)
		wg.Done()
	}
	log.Debug("file sent")

	//closing wtite stream
	err = stream.CloseWrite()
	if err != nil {
		log.Error("can't close write stream. ", err)
		stream.Reset()
		return nil, err
	}
	log.Debug("stream write closed succsesfuly")

	//reading stream for cid
	buf2, err := ioutil.ReadAll(stream)
	if err != nil {
		log.Error("cant read cid from stream.")
		stream.Reset()
		return nil, err
	}
	id := string(buf2)
	log.Debugf("received cid: %s", id)
	err = stream.CloseRead()
	if err != nil {
		log.Error("failed to close read stream. ", err)
		stream.Reset()
		return nil, err
	}
	stream.Close()
	return &id, nil
}
