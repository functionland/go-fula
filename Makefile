GO_MOD_REPLACEMENT=replace github.com/raulk/go-watchdog => ./tmp-fula-build/go-watchdog

all:
	go test ./...

fula-xcframework: patch-go-watchdog prepare-gomobile build-fula-xcframework bundles

patch-go-watchdog:
	mkdir -p tmp-fula-build &&\
	cd tmp-fula-build &&\
	git clone https://github.com/status-im/go-watchdog &&\
	cd go-watchdog &&\
	git checkout v1.2.0-ios-nolibproc &&\
	cd ../.. &&\
	echo "$(GO_MOD_REPLACEMENT)" >> go.mod &&\
	go mod tidy

prepare-gomobile:
	go install golang.org/x/mobile/cmd/gomobile@latest &&\
	go get -u golang.org/x/mobile/bind && go install golang.org/x/mobile/bind 

build-fula-xcframework:
	sudo xcode-select -r &&\
	gomobile bind -v -o Fula.xcframework -target=ios github.com/functionland/go-fula/mobile

clean-up:
	grep -v "$(GO_MOD_REPLACEMENT)" ./go.mod > ./tmp.mod ; mv ./tmp.mod ./go.mod &&\
	rm -rf ./tmp-fula-build
	go mod tidy
