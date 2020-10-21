package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/DigitalUnion/dp_aware_demon/awarent"
	"github.com/DigitalUnion/dp_aware_demon/handlers"

	"github.com/gin-gonic/gin"
)

var (
	cfgFile string // 配置文件
)

func main() {

	//初始化 awarent  服务注册，加载限流规则和IPFilter规则，服务监听，动态规则更新
	//ServiceName: 服务名字
	//Port: 运行端口
	//Nacos: nacos ip和端口
	//Group: 组
	//ConfigID: 配置文件的dataid
	//RuleID:   规则的dataid
	aware, err := awarent.InitAwarent(awarent.Config{
		ServiceName: "ddv",
		Port:        8080,
		Nacos: awarent.Nacos{
			IP:   "192.168.1.71",
			Port: 8848,
		},
		Group: "DDV_TEST",
		// ConfigID: "DDV_CONFIG",
		RuleID: "DDV_RULES",
	})
	if err != nil {
		panic("init awarent client error")
	}

	e := gin.New()
	e.Use(gin.Recovery())
	//gin 使用 IP过滤middleware
	e.Use(aware.IPFilter())
	//gin 使用 限流middleware
	e.Use(aware.Sentinel())
	e.GET("/", func(c *gin.Context) {
		c.String(200, "OK")
	})
	e.HEAD("/", func(c *gin.Context) {
		c.AbortWithStatus(200)
	})
	//gin 使用prometheus监控 包含限流统计
	e.GET("/awarent", awarent.PromHandler)
	e.GET("/q", handlers.GetDDV)
	srv := &http.Server{
		Addr:    "0.0.0.0:8080",
		Handler: e,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			fmt.Printf("start server error:%v\n", err)
		}
	}()
	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	<-quit
	//
	aware.Deregister()
}
