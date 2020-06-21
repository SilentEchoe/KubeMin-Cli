cd Models/protos
protoc --micro_out=../ --go_out=../ Prods.proto
protoc-go-inject-tag -input=../Prods.pb.go
cd .. && cd ..