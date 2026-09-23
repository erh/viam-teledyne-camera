.PHONY: build clean

build:
	mkdir -p build
	go build -o build/flir-camera ./cmd/module/

module.tar.gz: build
	tar czf module.tar.gz -C build flir-camera -C .. run.sh

clean:
	rm -rf build module.tar.gz
