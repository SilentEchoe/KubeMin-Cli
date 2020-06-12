package main

import (
	"fmt"
	"github.com/micro/go-micro/client/selector"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-plugins/registry/consul"
	"io/ioutil"
	"log"
	"net/http"

	"time"

)

func main()  {
	consulReg := consul.NewRegistry(
		registry.Addrs("http://127.0.0.1:8500/"),
	)


	for {
		getService,err := consulReg.GetService("prodservice")
		if err != nil {
			log.Fatal(err)
		}
		// 随机获取
		//next := selector.Random(getService)

		next := selector.RoundRobin(getService)
		node,err :=  next()
		if err != nil {
			log.Fatal(err)
		}

		callRes,err := callAPI(node.Address,"/v1/prods","GET")
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(callRes)
		time.Sleep(time.Second*1)
	}



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