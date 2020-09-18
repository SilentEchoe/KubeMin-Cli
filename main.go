package main

import (
	"LearningNotes-GoMicro/models"
	"LearningNotes-GoMicro/pkg/logging"
	"LearningNotes-GoMicro/pkg/setting"
	"LearningNotes-GoMicro/routers"
	"github.com/micro/go-micro/registry"
	"LearningNotes-GoMicro/pkg/gredis"
	"github.com/micro/go-micro/web"
	"github.com/micro/go-plugins/registry/consul"

	_ "github.com/go-sql-driver/mysql"
)

func main() {

	consulReg := consul.NewRegistry( //新建一个consul注册的地址，也就是我们consul服务启动的机器ip+端口
		registry.Addrs("127.0.0.1:8500"),
	)



	ginRouter := routers.InitRouter()
	httpServer := web.NewService(
		web.Name("HttpMainservice"),
		web.Handler(ginRouter),
		web.Address(":8000"),
		web.Registry(consulReg),
	)

	httpServer.Init() //加了这句就可以使用命令行的形式去设置我们一些启动的配置
	httpServer.Run()

}
