package main

import "grpc-server/pb"

var  employees = []pb.Employee {
	{
		Id :1,
		No : 1994,
		FirstName : "Chandler",
		LastName : "Bing",
		MonthSalay : &pb.MonthSalary{
			Basic : 5000,
			Bonus : 125.5,
		},
		Status : pb.EmployeeStatus_NORMAL,
	},
	{
		Id :1,
		No : 1994,
		FirstName : "Chandler",
		LastName : "Bing",
		MonthSalay : &pb.MonthSalary{
			Basic : 5000,
			Bonus : 125.5,
		},
		Status : pb.EmployeeStatus_NORMAL,
	},
}