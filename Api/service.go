package main

import (
	//"LearningNotes-GoMicro/Helper"
	//"LearningNotes-GoMicro/ProdService"
	//"LearningNotes-GoMicro/ServiceImpl"
	"LearningNotes-GoMicro/Services"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/micro/go-micro"
	"github.com/micro/go-micro/registry"
	//"github.com/micro/go-micro/registry/etcd"
	"github.com/micro/go-plugins/registry/consul"

	//"github.com/micro/go-micro/server"

	"github.com/micro/go-micro/web"
	"time"
)

func main() {
	consulReg := consul.NewRegistry( //新建一个consul注册的地址，也就是我们consul服务启动的机器ip+端口
		registry.Addrs("127.0.0.1:8500"),
	)

	/*etcdReg := etcd.NewRegistry( //新建一个consul注册的地址，也就是我们consul服务启动的机器ip+端口
		registry.Addrs("127.0.0.1:2380"),
	)*/
	ginRouter := gin.Default()

	//其实下面这段代码的意义就是启动服务的同时把服务注册进consul中，做的是服务发现
	server := web.NewService( //go-micro很灵性的实现了注册和反注册，我们启动后直接ctrl+c退出这个server，它会自动帮我们实现反注册
		web.Name("httpprodservice"), //注册进consul服务中的service名字
		web.Address(":8081"), //注册进consul服务中的端口,也是这里我们gin的server地址
		web.Handler(ginRouter),  //web.Handler()返回一个Option，我们直接把ginRouter穿进去，就可以和gin完美的结合
		web.Metadata(map[string]string{"protocol" : "http"}),
		web.Registry(consulReg), //注册到哪个服务器伤的consul中
		web.RegisterTTL(time.Second*30),
		web.RegisterInterval(time.Second*15),
	)
	myService := micro.NewService(micro.Name("prodservice.client"))
	prodService := Services.NewProdService("prodservice",myService.Client())

	fmt.Println(prodService)

	v1Group := ginRouter.Group("/v1")
	{
		v1Group.Handle("POST","/prods",  func(ginCtx *gin.Context) {
			/*var pr Helper.ProdsRequest
			err := context.Bind(&pr)
			if err != nil || pr.Size <=0 {
				pr= Helper.ProdsRequest{Size:2}
			}*/

			var prodReq Services.ProdsRequest
			err := ginCtx.Bind(&prodReq)
			if err !=nil {
				fmt.Println(err)
				ginCtx.JSON(500,gin.H{"status":err.Error()})
			}else {
			 prodRes,_ :=	prodService.GetProdsList(context.Background(),&prodReq)

				ginCtx.JSON(200,gin.H{"data":prodRes.Data})
			}


	})}

	server.Init() //加了这句就可以使用命令行的形式去设置我们一些启动的配置
	server.Run()
}