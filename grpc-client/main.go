package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"grpc-client/pb"
	"io"
	"log"
	"os"
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
	//getAll(client)
	addPhoto(client)
}

func addPhoto(client pb.EmployeeServiceClient)  {
	imgFile ,err := os.Open("wlop.jpg")
	if err != nil {
		log.Fatal(err.Error())
	}
	defer  imgFile.Close()

	md := metadata.New(map[string]string{"no" : "1994"})

	context  := context.Background()
	context = metadata.NewOutgoingContext(context,md)

	stream,err := client.AddPhoto(context)

	if err!=nil {
		log.Fatal(err.Error())
	}

	for {
		chunk := make([]byte, 128*1024)
		chunkSize ,err := imgFile.Read(chunk)

		if err == io.EOF {
			break
		}

		if err !=nil{
			log.Fatal(err.Error())
		}

		if chunkSize < len(chunk) {
			chunk = chunk[:chunkSize]
		}
		stream.Send(&pb.AddPhotoRequest{Data:chunk})

	}

	res ,err := stream.CloseAndRecv()
	if err!=nil {
		log.Fatal(err.Error())
	}

	fmt.Println(res.IsOk)

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



