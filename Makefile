OUT_DIR=./out
MKDIR_P = mkdir -p
PATH := $(PATH):$(go env GOPATH)/bin

mobile-build:
	gomobile bind -ldflags='-s -w' -v  -o $(OUT_DIR)/fula-arm64.aar -target=android/arm64 github.com/functionland/go-fula/mobile
	gomobile bind -ldflags='-s -w' -v  -o $(OUT_DIR)/fula-x86.aar -target=android/amd64 github.com/functionland/go-fula/mobile

$(OUT_DIR):
	${MKDIR_P} $(OUT_DIR)

directories: $(OUT_DIR)	

all: directories mobile-build

.PHONY: directories all clean
clean:
	rm -rf $(OUT_DIR)