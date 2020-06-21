cd Models/protos
protoc --micro_out=../ --go_out=../ Prods.proto
cd .. && cd ..