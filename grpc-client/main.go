package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"grpc-client/pb"
	"io"
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
	client := pb.NewEmployeeServiceClient(conn)
	//getByNo(client)
	getAll(client)
}

func getAll(client pb.EmployeeServiceClient)  {
	stream , err := client.GetAll(context.Background(),&pb.GetAllRequest{})
	if err!=nil {
		log.Fatal(err.Error())
	}

	for{
		res ,err :=stream.Recv()
		if err == io.EOF {
			break;
		}
		if err!=nil {
			log.Fatal(err.Error())
		}

		fmt.Println(res.Employee)
	}

}


func getByNo(client pb.EmployeeServiceClient)  {
	res,err := client.GetByNo(context.Background(), &pb.GetByNoRequest{No:1994})
	if err !=nil{
		log.Fatal(err.Error())
	}
	fmt.Println(res.Employee)

}



