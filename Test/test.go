package main

import (
	"fmt"
	"github.com/micro/go-micro/client/selector"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-plugins/registry/consul"
	"log"
)

func main()  {
	consulReg := consul.NewRegistry(
		registry.Addrs("http://127.0.0.1:8500/"),
	)
	 getService,err := consulReg.GetService("prodservice")
	if err != nil {
		log.Fatal(err)
	}

	// 随机获取
	next := selector.Random(getService)
	node,err :=  next()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(node.Id,node.Address,node.Metadata)

}
