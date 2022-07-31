package file

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"io/ioutil"

	files "github.com/ipfs/go-ipfs-files"
	coreiface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/network"
	"google.golang.org/protobuf/proto"
)

func RequestHandler(ctx context.Context, api coreiface.CoreAPI, stream network.Stream) {
	pref_buf := make([]byte, binary.MaxVarintLen64)
	pref_reader := bytes.NewReader(pref_buf)
	n, err := io.ReadFull(stream, pref_buf)
	if err != nil {
		log.Error("fail to read perfix", err)
		stream.Reset()
		return
	}
	log.Debug("read %d byte from stream", n)
	msg_size, err := binary.ReadUvarint(pref_reader)
	if err != nil {
		log.Error("fail to pars perfix", err)
		stream.Reset()
		return
	}
	log.Debug("readed message size from perfix ", msg_size)
	header, err := io.ReadAll(pref_reader)
	msg_buf := make([]byte, int(msg_size)-len(header))
	if err != nil {
		log.Error("fail to read header", err)
		stream.Reset()
		return
	}
	req := &Request{}
	n, err = io.ReadFull(stream, msg_buf)
	if err != nil {
		log.Error("fail to read header", err)
		stream.Reset()
		return
	}
	log.Debug("readed message with size ", n)
	header = append(header, msg_buf...)
	err = proto.Unmarshal(header, req)
	if err != nil {
		log.Error("can't unmarshal header bytes ", err)
		stream.Reset()
		return
	}
	switch v := req.Type.(type) {
	case *Request_Receive:
		ReceiveFileHandler(v, api, stream, ctx)
	case *Request_Meta:
		ReceiveMetaHandler(v, api, stream, ctx)
	case *Request_Send:
		SendFileHandler(v, api, stream, ctx)
	default:
		log.Error("message not supported")
	}
}

func ReceiveFileHandler(req *Request_Receive, api coreiface.CoreAPI, stream network.Stream, ctx context.Context) {
	file_path := path.New("/ipfs/" + req.Receive.Id)
	f, err := api.Unixfs().Get(ctx, file_path)
	if err != nil {
		log.Error("cant not resolve file ", err)
		stream.Reset()
		return
	}
	var fileNode files.File
	switch f := f.(type) {
	case files.File:
		fileNode = f
	case files.Directory:
		log.Error("not file")
		return
	default:
		log.Error("not file")
		return
	}
	fBytes, err := ioutil.ReadAll(fileNode)
	if err != nil {
		log.Error("error reading file ", err)
		stream.Reset()
		return
	}
	fileProto := &File{}
	err = proto.Unmarshal(fBytes, fileProto)
	if err != nil {
		log.Error("connot unmarshal file", err)
		stream.Reset()
		return
	}
	content_path := path.New(fileProto.GetContentPath())
	c, err := api.Unixfs().Get(ctx, content_path)
	if err != nil {
		log.Error("cant not resolve file ", err)
		stream.Reset()
		return
	}
	var fileNode1 files.File
	switch c := c.(type) {
	case files.File:
		fileNode1 = c
	case files.Directory:
		log.Error("not file")
		return
	default:
		log.Error("not file")
		return
	}
	fBytes1, err := ioutil.ReadAll(fileNode1)
	log.Info("size of out file:  ", len(fBytes1))
	if err != nil {
		log.Error("error reading file ", err)
		stream.Reset()
		return
	}
	n, err := stream.Write(fBytes1)
	if err != nil {
		log.Error("can not write to stream ", err)
		stream.Reset()
		return
	}
	log.Info("write size to stream :  ", n)
	err = stream.CloseWrite()
	if err != nil {
		log.Error("can not close stream ", err)
		stream.Reset()
		return
	}
}

func ReceiveMetaHandler(req *Request_Meta, api coreiface.CoreAPI, stream network.Stream, ctx context.Context) {
	path := path.New("/ipfs/" + req.Meta)
	f, err := api.Unixfs().Get(ctx, path)
	if err != nil {
		log.Error("cant not resolve file ", err)
		stream.Reset()
		return
	}
	var fileNode files.File
	switch f := f.(type) {
	case files.File:
		fileNode = f
	case files.Directory:
		log.Error("not file")
		stream.Reset()
		return
	default:
		log.Error("not file")
		stream.Reset()
		return
	}
	fBytes, err := ioutil.ReadAll(fileNode)
	if err != nil {
		log.Error("error reading file ", err)
		stream.Reset()
		return
	}
	fileProto := &File{}
	err = proto.Unmarshal(fBytes, fileProto)
	if err != nil {
		log.Error("connot unmarshal file", err)
		stream.Reset()
		return
	}
	meta, err := proto.Marshal(fileProto.Meta)
	if err != nil {
		log.Error("connot marshal meta", err)
		stream.Reset()
		return
	}
	n, err := stream.Write(meta)
	if err != nil {
		log.Error("can not write to stream", err)
	}
	log.Debugf("wrote meta with size %d on stream", n)
	err = stream.CloseWrite()
	if err != nil {
		log.Error("can not close stream ", err)
		stream.Reset()
		return
	}
}

func SendFileHandler(req *Request_Send, api coreiface.CoreAPI, stream network.Stream, ctx context.Context) {
	fBytes, err := ioutil.ReadAll(stream)
	log.Info("size of input file:  ", len(fBytes))
	if err != nil {
		log.Error("stream error ", err)
		stream.Reset()
		return
	}
	file_cid, err := api.Unixfs().Add(ctx, files.NewBytesFile(fBytes))
	log.Debug("file added with cid ", file_cid)
	if err != nil {
		log.Error("write file to ipfs failed ", err)
		stream.Reset()
		return
	}
	fileProto := &File{Meta: req.Send, ContentPath: file_cid.String()}
	log.Debug("FileLink added", fileProto)
	fileLinkByte, err := proto.Marshal(fileProto)
	if err != nil {
		log.Error("creating proto of file link failed ", err)
		stream.Reset()
		return
	}
	filelink_cid, err := api.Unixfs().Add(ctx, files.NewBytesFile(fileLinkByte))
	if err != nil {
		log.Error("can not write the filelink to ipfs ", err)
		stream.Reset()
		return
	}
	log.Debug("FileLink added and with cid ", filelink_cid)

	n, err := stream.Write([]byte(filelink_cid.Cid().Hash().B58String()))
	if err != nil {
		log.Error("can not write cid to stream", err)
		stream.Reset()
	}
	log.Debugf("wrote cid with size %d on stream", n)
	err = stream.CloseWrite()
	if err != nil {
		log.Error("failed to write cid to stream", err)
		stream.Reset()
	}
}
