package main

import (
	"LearningNotes-GoMicro/pb"
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
	creds, err := credentials.NewClientTLSFromFile("cert.pem","ket.pem")
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

func (s *employeeService) GetByNo(context.Context, *pb.GetByNoRequest) (*pb.EmployeeResponse, error)  {
	return  nil , nil
}


func (s *employeeService) Save (context.Context, *pb.EmployeeRequest)(*pb.EmployeeResponse,error) {
	return  nil,nil
}


