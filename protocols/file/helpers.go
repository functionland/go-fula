package file;

import (
	"os"
	"github.com/gabriel-vasile/mimetype"
)

type IFileMeta interface{
	toMetaProto()
}

type FileMeta struct {
	name string
	size int64
	lastModified int64
	mtype string
}

func (m *FileMeta) ToMetaProto() Meta {
	return Meta{
		Name:         m.name,
		Size_:        uint64(m.size),
		LastModified: m.lastModified,
		Type:         m.mtype}
}

func FromFile(path string) (*FileMeta, error) {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	mtype, err := mimetype.DetectFile(path)
	if err != nil {
		return nil, err
	}
	return &FileMeta{
		name: fileInfo.Name(),
		size:fileInfo.Size(), 
		lastModified: fileInfo.ModTime().Unix(),
		mtype: mtype.String()},nil
}