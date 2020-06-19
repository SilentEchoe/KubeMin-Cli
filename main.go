package main

import (
	"LearningNotes-GoMicro/Helper"
	"LearningNotes-GoMicro/ProdService"
	"github.com/gin-gonic/gin"
	"github.com/micro/go-micro/registry/etcd"
	"github.com/micro/go-micro/web"

	"github.com/micro/go-micro/registry"

	_ "github.com/micro/go-plugins/registry/consul"
)

func main()  {
	etcdReg := etcd.NewRegistry(
		registry.Addrs("http://127.0.0.1:2379/"),
	)

	ginRouter := gin.Default()
	v1Group := ginRouter.Group("/v1")
	{
		v1Group.Handle("POST","/prods",  func(context *gin.Context) {
			var pr Helper.ProdsRequest
			err := context.Bind(&pr)
			if err != nil || pr.Size <=0 {
				pr= Helper.ProdsRequest{Size:2}
			}

			context.JSON(200,
				gin.H{
					"data":ProdService.NewProdList(pr.Size)})
		})
	}


	ginRouter.Handle("GET","/user", func(context *gin.Context) {
		context.String(200,"user api")
	})

	server := web.NewService(
		web.Name("prodservice"),
		//web.Address(":8001"),
		web.Handler(ginRouter),
		web.Registry(etcdReg),
	)




	server.Init()
	server.Run()



}
