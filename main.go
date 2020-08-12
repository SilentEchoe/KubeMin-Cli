package main

import (
	"LearningNotes-GoMicro/models"
	"LearningNotes-GoMicro/pkg/logging"
	"LearningNotes-GoMicro/pkg/setting"
	"LearningNotes-GoMicro/routers"
	"github.com/micro/go-micro/registry"
	"github.com/micro/go-micro/web"
	"github.com/micro/go-plugins/registry/consul"

	_ "github.com/go-sql-driver/mysql"
	"fmt"
)

func main() {
	consulReg := consul.NewRegistry( //新建一个consul注册的地址，也就是我们consul服务启动的机器ip+端口
		registry.Addrs("127.0.0.1:8500"),
	)
	setting.Setup()
	fmt.Println("a")

	models.Setup()
	logging.Setup()

	ginRouter := routers.InitRouter()

	httpServer := web.NewService(
		//注册进consul服务中的service名字
		web.Name("httpprodservice"),
		//web.Handler()返回一个Option，我们直接把ginRouter穿进去，就可以和gin完美的结合
		web.Handler(ginRouter),
		//注册进consul服务中的端口,也是这里我们gin的server地址
		web.Address(":8000"),
		web.Registry(consulReg),
	)

	httpServer.Init() //加了这句就可以使用命令行的形式去设置我们一些启动的配置
	httpServer.Run()





}
