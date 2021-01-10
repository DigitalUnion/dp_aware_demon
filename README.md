# du_aware_demon
 1. flowcontrol 
 2. ipfilter 
 3. service register/deregister
 4. configure dynamic get/upgrade
 5. dynamic upgrade flowcontrol/ipfilter rule
 6.  service client side load balance


# 架构
![architecture](./dp_aware_demon.png)


### config.yaml

```yaml
resource-param: user_id
flow-control-rules:
  - resource: /active/applist/and/1
    threshold: 100
  - resource: /active/apps/and/1
    threshold: 50
  - resource: /active/devices/and/1
    threshold: 1000
ip-filter-rules:
  allowed:
    - 127.0.0.1
    - localhost
    - 172.17.130.223
  blocked:
    - 10.10.1.21
    - 192.168.1.24
  urlPath: /active
  urlParam: ":user_id"
  blockedDefault: false
  authorized:
    - resource: "user_id:1"
      ips:
        - 127.0.0.1
        - localhost
        - 172.17.130.223
    - resource: "user_id:2"
      ips:
        - 127.0.0.1
        - localhost
        - 172.17.130.223

day-flow-rules:
  - resource: 1
    threshold: 100
  - resource: 2
    threshold: 1000

```



### init awarent
 

	ServiceName: 服务名字 

	Port： 服务端口 
  
	Nacos： nacos配置 包括 IP，Port 
  
	Group： 组，服务所在组 例如 DDV_TEST,DDV_DEV,DDV_PROD 
  
	ConfigID: nasco 配置 dataid 
  
	RuleID： IP Filter， 流量控制规则ID 


```
	aware, err := awarent.InitAwarent(awarent.Config{
		ServiceName: "ddv",
		Port:        8080,
		Nacos: awarent.Nacos{
			IP:   "192.168.1.71",
			Port: 8848,
		},
		Group: "DDV_TEST",
		RuleID: "DDV_RULES",// 可选，如果RuleID为空 则 限流和IP过滤功能将不可用
	})
```



### gin 使用 awarent

```
    e := gin.New()
	e.Use(gin.Recovery())
	//gin 使用 IP过滤middleware
	e.Use(aware.IPFilter())
	//gin 使用 限流middleware
	e.Use(aware.Sentinel())
	//gin 使用prometheus监控 包含限流统计
	e.GET("/awarent", awarent.PromHandler)
	//获取配置  
	content, _ := aware.GetConfig("DDV_CONFIG")
	fmt.Printf("content:%s", content)
	//监听配置更新 动态配置
	aware.ConfigOnChange("DDV_CONFIG", func(data string) {
		fmt.Printf("config updated:%s\n", data)
	})
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
	//服务注销
	aware.Deregister()
```


### nacos docker-compose 安装

* Clone 项目 并且进入项目根目录

  ```powershell
  git clone https://github.com/paderlol/nacos-docker.git
  cd nacos-docker
  ```


* 单机mysql8 

  ```powershell
  docker-compose -f example/standalone-mysql-8.yaml up -d 
  ```
  

