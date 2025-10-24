.PHONY: build run clean deps

build:
	go build -o OsuCollectionTab.exe main.go

run:
	go run main.go

clean:
	rm -rf OsuCollectionTab.exe

deps:
	go mod tidy