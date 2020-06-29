package main

import (
	"LearningNotes-GoMicro/ServiceImpl"
	"LearningNotes-GoMicro/Services"
	"github.com/micro/go-micro"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-plugins/registry/consul"
)

func main()  {

	consulReg := consul.NewRegistry(
		registry.Addrs("http://127.0.0.1:8500"),
	)

	prodService := micro.NewService(
		micro.Name("prodservice"),
		micro.Address(":8011"),
		micro.Registry(consulReg),
		)
	prodService.Init()
	Services.RegisterProdServiceHandler(prodService.Server(),new(ServiceImpl.ProdService))
	prodService.Run()
}