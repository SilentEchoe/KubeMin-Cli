package main

import (
	"LearningNotes-GoMicro/Models"
	"context"
	"fmt"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-micro/registry/etcd"
	//"github.com/micro/go-plugins/registry/consul"

	"github.com/micro/go-micro/client"
	"log"

	"github.com/micro/go-micro/client/selector"
	myhttp "github.com/micro/go-plugins/client/http"


)

func main() {
	/*consulReg := consul.NewRegistry(
		registry.Addrs("http://127.0.0.1:8500"),
	)*/

	etcdReg := etcd.NewRegistry(registry.Addrs("127.0.0.1:2380"))

	mySelector := selector.NewSelector(
		selector.Registry(etcdReg),
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
	//req := myClient.NewRequest("httpprodservice","/v1/prods",map[string]int {"size":4})
	req := myClient.NewRequest("httpprodservice","/v1/prods",
		Models.ProdsRequest{Size: 6})

	var rsp Models.ProdListResponse

	//var rsp map[string]interface{}
	err := myClient.Call(context.Background(),req,&rsp)

	if err!=nil {
		log.Fatal(err)
	}
	fmt.Print(rsp)
	fmt.Println(rsp.GetData())

}


