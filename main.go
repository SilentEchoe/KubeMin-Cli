package main

import (
	"LearningNotes-GoMicro/pb"
	"context"
)

func main()  {
	
}

type employeeService struct {}

func (s *employeeService) GetByNo(context.Context, *pb.GetByNoRequest) (*pb.EmployeeResponse, error)  {
	return  nil , nil
}
