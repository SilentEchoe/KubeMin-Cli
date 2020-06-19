package main

import (
	"context"
	"fmt"
	"github.com/micro/go-micro/registry"

	"github.com/micro/go-micro/client"
	"log"

	"github.com/micro/go-micro/client/selector"
	"github.com/micro/go-micro/registry/etcd"


	myhttp "github.com/micro/go-plugins/client/http"

	"io/ioutil"

	"net/http"
)

func main() {
	/*consulReg := consul.NewRegistry(
		registry.Addrs("http://127.0.0.1:8500"),
	)*/

	etcdReg := etcd.NewRegistry(registry.Addrs("http://127.0.0.1:8500"))

	mySelector := selector.NewSelector(
		selector.Registry(etcdReg),
		selector.SetStrategy(selector.RoundRobin),
	)

	callAPI2(mySelector)

}

func callAPI2(s selector.Selector)  {

	myClient := myhttp.NewClient(

		client.Selector(s),
		client.ContentType("application/json"),
		)
	fmt.Println(myClient.String())
	req := myClient.NewRequest("prodservice","/v1/user",map[string]int {"Size":4})
	fmt.Println(req)
	var rsp map[string]interface{}
	err := myClient.Call(context.Background(),req,&rsp)
	if err!=nil {
		log.Fatal(err)
	}

	fmt.Println(rsp["data"])

}


func callAPI(addr string,path string,method string) (string,error){
	req ,_ := http.NewRequest(method,"http://" + addr + path,nil)
	client := http.DefaultClient
	res,err := client.Do(req)
	if err!=nil {
		return  "",err
	}

	defer  res.Body.Close()
	buf,err := ioutil.ReadAll(res.Body)
	return  string(buf),nil

}