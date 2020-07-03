cd Models/protos
protoc --micro_out=../ --go_out=../ Prods.proto
protoc-go-inject-tag -input=../Prods.pb.go
cd .. && cd ..

cd Services/protos
protoc --micro_out=../ --go_out=../ Models.proto
protoc --micro_out=../ --go_out=../ ProdService.proto
protoc-go-inject-tag -input=../Models.pb.go
protoc-go-inject-tag -input=../ProdService.pb.go
cd .. && cd ..