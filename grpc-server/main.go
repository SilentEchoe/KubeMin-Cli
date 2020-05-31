package main

import (
	"fmt"
	"google.golang.org/grpc/metadata"
	"grpc-server/pb"
	"errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"log"
	"net"
)

const  port =  ":5001"



func main()  {
	Listen,err := net.Listen("tcp", port)
	if err!=nil {
		log.Fatalln(err.Error())
	}
	creds, err := credentials.NewServerTLSFromFile("cert.pem","key.pem")
	if err != nil {
		log.Fatalln(err.Error())
	}
	options := []grpc.ServerOption{grpc.Creds(creds)}
	server := grpc.NewServer(options ...)
	pb.RegisterEmployeeServiceServer(server, new(employeeService))


	log.Println("Grpc Server started... " + port)
	server.Serve(Listen)


}

type employeeService struct {}

func (s *employeeService) GetAll(req *pb.GetAllRequest,stream pb.EmployeeService_GetAllServer) error {
	for _, e := range  employees {
		stream.Send(& pb.EmployeeResponse{Employee: &e})
	}
	return  nil
}

func (s *employeeService) AddPhoto(stream pb.EmployeeService_AddPhotoServer) error {

	md, ok := metadata.FromIncomingContext(stream.Context())

	if ok{
		// 服务端收到的KEY 一定要是小写
		fmt.Println("Employee: %s\n", md["no"][0])
	}

	img := []byte{}
	for {
		data, err := stream.Recv()
		// 判断客户端是否关闭
		if err == io.EOF {
			fmt.Printf("File Size: %d\n ", len(img))
		return  stream.SendAndClose(& pb.AddPhotoResponse{IsOk:true})
		}
		if err !=nil {
			return err
		}


		fmt.Printf("File receuved:%d\n",len(data.Data))
		img = append(img,data.Data...)

	}

	panic("implement me")
}

func (s *employeeService) SaveAll(pb.EmployeeService_SaveAllServer) error {
	panic("implement me")
}

func (s *employeeService) GetByNo(ctx context.Context, req *pb.GetByNoRequest) (*pb.EmployeeResponse, error)  {

	for _, e:= range employees {
		if req.No == e.No {
			return &pb.EmployeeResponse{
				Employee: &e,
			}, nil
		}
	}


	return  nil , errors.New("Employee not found")
}


func (s *employeeService) Save (context.Context, *pb.EmployeeRequest)(*pb.EmployeeResponse,error) {
	return  nil,nil
}


