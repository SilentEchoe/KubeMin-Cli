package main

import (
	"context"
	"fmt"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-plugins/registry/consul"

	"github.com/micro/go-micro/client"
	"log"

	"github.com/micro/go-micro/client/selector"
	//"github.com/micro/go-micro/registry/etcd"


	myhttp "github.com/micro/go-plugins/client/http"


)

func main() {
	consulReg := consul.NewRegistry(
		registry.Addrs("http://127.0.0.1:8500"),
	)

	//etcdReg := etcd.NewRegistry(registry.Addrs("http://127.0.0.1:8500"))

	mySelector := selector.NewSelector(
		selector.Registry(consulReg),
		selector.SetStrategy(selector.RoundRobin),
	)

	callAPI(mySelector)

}

func callAPI(s selector.Selector)  {

	myClient := myhttp.NewClient(

		client.Selector(s),
		client.ContentType("application/json"),
	)
	fmt.Println(myClient.String())
	req := myClient.NewRequest("httpprodservice","/v1/prods",map[string]int {"size":4})
	fmt.Println(req)
	var rsp map[string]interface{}
	err := myClient.Call(context.Background(),req,&rsp)
	if err!=nil {
		log.Fatal(err)
	}

	fmt.Println(rsp["data"])

}


