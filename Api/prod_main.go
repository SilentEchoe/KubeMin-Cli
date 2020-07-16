package main

import (
	"LearningNotes-GoMicro/Services"
	"LearningNotes-GoMicro/Weblib"
	//"github.com/gin-gonic/gin"
	"github.com/micro/go-micro"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-micro/web"
	"github.com/micro/go-plugins/registry/consul"
)

func main() {
	consulReg := consul.NewRegistry( //新建一个consul注册的地址，也就是我们consul服务启动的机器ip+端口
		registry.Addrs("127.0.0.1:8500"),
	)

	myService := micro.NewService(micro.Name("prodservice.client"))

	prodService := Services.NewProdService("prodservice",myService.Client())
	print(prodService)
    httpServer := web.NewService(
		web.Name("httpprodservice"), //注册进consul服务中的service名字
		web.Address(":8001"), //注册进consul服务中的端口,也是这里我们gin的server地址
		web.Handler(Weblib.NewGinRouter(prodService)),
		web.Registry(consulReg),
    	)




	httpServer.Init() //加了这句就可以使用命令行的形式去设置我们一些启动的配置
	httpServer.Run()

	//其实下面这段代码的意义就是启动服务的同时把服务注册进consul中，做的是服务发现
	/*server := web.NewService( //go-micro很灵性的实现了注册和反注册，我们启动后直接ctrl+c退出这个server，它会自动帮我们实现反注册
		web.Name("httpprodservice"), //注册进consul服务中的service名字
		web.Address(":8081"), //注册进consul服务中的端口,也是这里我们gin的server地址
		web.Handler(ginRouter),  //web.Handler()返回一个Option，我们直接把ginRouter穿进去，就可以和gin完美的结合
		web.Metadata(map[string]string{"protocol" : "http"}),
		web.Registry(consulReg), //注册到哪个服务器伤的consul中
		web.RegisterTTL(time.Second*30),
		web.RegisterInterval(time.Second*15),
	)*/





}