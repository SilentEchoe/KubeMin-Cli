package main

import (
	"context"
	"grpc-client/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log"
)

const port = ":5001"

func main()  {
	creds,err := credentials.NewClientTLSFromFile("cert.pem","")
	if err!=nil {
		log.Fatal(err.Error())
	}
	options := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	conn,err := grpc.Dial("localhost" + port, options ...)
	if err!=nil {
		log.Fatal(err.Error())
	}
	defer conn.Close()
	client =  pb.NewEmployeeServiceClient(conn)


}

func GetByNo(client pb.EmployeeServiceClient)  {
	res,err := client.GetByNo(context.Background(),)
}

