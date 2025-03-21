package main

import (
	"fmt"

	"github.com/BitofferHub/msgcenter/src/config"
	"github.com/BitofferHub/msgcenter/src/ctrl/consumer"
	"github.com/BitofferHub/msgcenter/src/data"
	"github.com/BitofferHub/msgcenter/src/initialize"
	"github.com/BitofferHub/pkg/middlewares/log"
	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化配置
	config.Init()
	_, err := data.NewData(config.Conf)
	if err != nil {
		log.Errorf("initialize NewData err %s", err.Error())
		return
	}
	cs := consumer.NewMsgConsume()
	cs.Consume()
	var tmc consumer.TimerMsgConsume
	tmc.Consume()
	consumer.InitMsgProc()
	// 创建一个web服务
	router := gin.Default()
	// 这里跳进去就能看到有哪些接口
	initialize.RegisterRouter(router)
	fmt.Println("before router run")
	// 启动web server，这一步之后这个主协程启动会阻塞在这里，请求可以通过gin的子协程进来
	err = router.Run(fmt.Sprintf(":%d", config.Conf.Common.Port))
	fmt.Println(err)
}
