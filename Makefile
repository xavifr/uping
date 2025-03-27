build:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -C src -ldflags "-w -s" -o ../bin/uping main.go


install: build
	cp bin/uping /usr/local/bin/uping
	chmod +x /usr/local/bin/uping
	setcap cap_net_raw=+ep /usr/local/bin/uping

run:
	go run -C src main.go $(PARAM)

check:
	@echo "Checking for code smell"
	@go run -C src honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	@echo "Checking for vulnerabilities"
	@go run -C src golang.org/x/vuln/cmd/govulncheck@latest ./...

binaries:
	@echo "Compiling for every OS and Platform"
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -C src -ldflags "-w -s" -o ../bin/uping-arm main.go
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -C src -ldflags "-w -s" -o ../bin/uping-arm64 main.go
	@CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -C src -ldflags "-w -s" -o ../bin/uping-x86 main.go
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -C src -ldflags "-w -s" -o ../bin/uping-x64 main.go
	@CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -C src -ldflags "-w -s" -o ../bin/uping-x86.exe main.go
	@CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -C src -ldflags "-w -s" -o ../bin/uping-x64.exe main.go

deb: build
	dpkg-buildpackage -us -uc -rfakeroot --build=binary

all: build
