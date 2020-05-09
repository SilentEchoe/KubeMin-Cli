package main

import (
	"grpc-server/pb"
	"errors"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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


	log.Println("Grpc Server started...")
	server.Serve(Listen)


}

type employeeService struct {}

func (s *employeeService) GetAll(context.Context, *pb.GetAllRequest) (*pb.EmployeeResponse, error) {
	panic("implement me")
}

func (s *employeeService) AddPhoto(pb.EmployeeService_AddPhotoServer) error {
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


