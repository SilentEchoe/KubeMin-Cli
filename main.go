package main

import (
	"LearningNotes-GoMicro/pb"
	"golang.org/x/net/context"
	"log"
	"net"
)

const  port =  ":5001"



func main()  {
	Listen,err := net.Listen("tcp", port)
	if err!=nil {
		log.Fatalln(err.Error())
	}




}

type employeeService struct {}

func (s *employeeService) GetByNo(context.Context, *pb.GetByNoRequest) (*pb.EmployeeResponse, error)  {
	return  nil , nil
}

func (s *employeeService) GetAll(request *pb.GetAllRequest,server pb.EmployeeService_SaveAllServer) error  {
	return  nil
}

func (s *employeeService)AddPhoto(pb.EmployeeService_SaveAllServer)  error {
	return  nil
}

func (s *employeeService) Save (context.Context, *pb.EmployeeRequest)(*pb.EmployeeResponse,error) {
	return  nil,nil
}


