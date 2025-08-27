build-apiserver:
	@go clean && GOOS=linux GOARCH=amd64 go build -o apiserver  -ldflags="-X main.BuildStamp=`date +%Y-%m-%d.%H:%M:%S`" pkg/apiserver/server.go