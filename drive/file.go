package drive

type FileType string

const (
	FILE      FileType = "file"
	DIRECTORY FileType = "directory"
)

type File struct {
	name     string
	size     int
	fileType FileType
	cid      string
}
