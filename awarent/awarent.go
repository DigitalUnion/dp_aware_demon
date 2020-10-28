package awarent

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/DigitalUnion/dp_aware_demon/balancer"
	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/flow"
	metric "github.com/alibaba/sentinel-golang/core/log/metric"
	"github.com/gin-gonic/gin"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/model"
	"github.com/nacos-group/nacos-sdk-go/util"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"gopkg.in/yaml.v2"
)

// Awarenter interface of awarent
type Awarenter interface {
	//Register register service
	Register() (bool, error)
	//Deregister unregister service
	Deregister() (bool, error)
	//GetConfig get config with configid from nacos
	GetConfig(configID string) (string, error)
	//ConfigOnChange listen on config change with callback function
	ConfigOnChange(configID string, callback ConfigChangeCallback) error

	ServiceClient(serviceName string, group string) (http.Client, error)
	GetService(serviceName string, group string) (model.Service, error)
}

//ConfigChangeCallback callback function when config changed
type ConfigChangeCallback func(data string)

//Config warentConfig entry struct
type Config struct {
	ServiceName string `yaml:"serviceName" toml:"serviceName" json:"serviceName"`
	Port        uint64 `yaml:"port" toml:"port" json:"port"`
	Group       string `yaml:"group" toml:"group" json:"group"`
	Nacos       Nacos  `yaml:"nacos" toml:"nacos" json:"nacos"`
	ConfigID    string `yaml:"configId" toml:"configId" json:"configId"`
	RuleID      string `yaml:"ruleId" toml:"ruleId" json:"ruleId"`
}

// Nacos config
type Nacos struct {
	IP   string `yaml:"ip" toml:"ip" json:"ip"`
	Port uint64 `yaml:"port" toml:"port" json:"port"`
}

//Awarent struct of awarent
type Awarent struct {
	serviceName  string
	port         uint64
	group        string
	logDir       string
	nacosIP      string
	nacosPort    uint64
	configID     string
	ruleID       string
	nameClient   naming_client.INamingClient
	configClient config_client.IConfigClient
	rule         Rule
}

//FlowControlOption option for flow control  resource for specify resource need to be controled, threshold, means every second passed request by flowcontrol. here means QPS
type FlowControlOption struct {
	Resource  string  `json:"resource"`
	Threshold float64 `json:"threshold"`
}

//Rule struct for flowcontrol/ipfilter rule collection.
type Rule struct {
	ResourceParam    string              `yaml:"resource-param"`
	FlowControlRules []FlowControlOption `yaml:"flow-control-rules"`
	IPFilterRules    FilterOptions       `yaml:"ip-filter-rules"`
}

//InitAwarent init awarent module
func InitAwarent(entity Config) (*Awarent, error) {
	logDir := os.TempDir() + string(os.PathSeparator) + entity.ServiceName

	awarent := &Awarent{
		serviceName: entity.ServiceName,
		group:       entity.Group,
		port:        entity.Port,
		logDir:      logDir,
		nacosIP:     entity.Nacos.IP,
		nacosPort:   entity.Nacos.Port,
		configID:    entity.ConfigID,
		ruleID:      entity.RuleID,
	}

	sentinelConfig := config.NewDefaultConfig()
	sentinelConfig.Sentinel.App.Name = entity.ServiceName
	sentinelConfig.Sentinel.Log.Dir = logDir
	sc := []constant.ServerConfig{
		{
			IpAddr: awarent.nacosIP,   //"192.168.1.71"
			Port:   awarent.nacosPort, //8848
		},
	}
	cc := constant.ClientConfig{
		TimeoutMs:           5000,
		ListenInterval:      10000,
		NotLoadCacheAtStart: true,
		LogDir:              sentinelConfig.Sentinel.Log.Dir,
		CacheDir:            sentinelConfig.Sentinel.Log.Dir,
		RotateTime:          "1h",
		MaxAge:              3,
		LogLevel:            "debug",
	}
	nameClient, err := clients.CreateNamingClient(map[string]interface{}{
		"serverConfigs": sc,
		"clientConfig":  cc,
	})
	if err != nil {
		return nil, err
	}
	awarent.nameClient = nameClient

	configClient, err := clients.CreateConfigClient(map[string]interface{}{
		"serverConfigs": sc,
		"clientConfig":  cc,
	})
	if err != nil {
		return nil, err
	}
	awarent.configClient = configClient
	sentinel.InitWithConfig(sentinelConfig)
	if len(awarent.ruleID) > 0 {
		awarent.loadRule(awarent.ruleID, true)
	}
	awarent.Register()
	awarent.Subscribe()
	return awarent, nil
}

func (a *Awarent) loadRule(ruleID string, listenOnChange bool) error {
	rc, err := a.GetConfig(ruleID)
	if err != nil {
		log.Printf("get flow control rule error:%v\n", err)
		return err
	}
	yamlDecoder := yaml.NewDecoder(strings.NewReader(rc))
	var rule Rule
	if err = yamlDecoder.Decode(&rule); err != nil {
		log.Printf("decode rule error:%v\n", err)
		return err
	}
	a.rule = rule
	log.Printf("load rules: %s\n", rc)
	a.loadFlowControlRules(rule.FlowControlRules...)
	if listenOnChange {
		ruleChangedCallback := func(data string) {
			log.Printf("ruleID:%s changed", ruleID)
			yamlDecoder := yaml.NewDecoder(strings.NewReader(data))
			var rule Rule
			if err := yamlDecoder.Decode(&rule); err != nil {
				log.Printf("decode yaml error:%v\n", err)
			}
			log.Printf("load rules:%s\n", data)
			a.rule = rule

			//reload rules
			a.loadFlowControlRules(rule.FlowControlRules...)
			//update ip filter rules
			updateIPFilter(rule.IPFilterRules)
		}
		a.ConfigOnChange(ruleID, ruleChangedCallback)
	}
	return nil
}

//Register register service
func (a *Awarent) Register() (bool, error) {
	regParam := vo.RegisterInstanceParam{
		ServiceName: a.serviceName,
		Ip:          util.LocalIP(),
		Port:        a.port,
		Weight:      10,
		Healthy:     true,
		Enable:      true,
		GroupName:   a.group,
	}

	return a.nameClient.RegisterInstance(regParam)
}

//GetService get single random service
func (a *Awarent) GetService(serviceName string, group string) (model.Service, error) {
	serviceParam := vo.GetServiceParam{
		ServiceName: serviceName,
		GroupName:   group,
	}
	return a.nameClient.GetService(serviceParam)
}

//ServiceClient return a httpclient for service. the httpclient auto balancer with roundrobin
func (a *Awarent) ServiceClient(serviceName string, group string) (*http.Client, error) {
	service, err := a.GetService(serviceName, group)
	if err != nil {
		return nil, err
	}
	instances := service.Hosts
	var urls []string
	if len(instances) > 0 {
		for _, instance := range instances {
			url := fmt.Sprintf("http://%s:%d", instance.Ip, instance.Port)
			urls = append(urls, url)
		}
	}
	bl, err := balancer.NewRoundrobinFromURL(urls...)
	if err != nil {
		fmt.Printf("new balancer error:%v\n", err)
		return nil, err
	}
	client := balancer.NewClient(bl)
	return client, nil
}

//Subscribe subscribe service change, do flow control re-balance flow control
func (a *Awarent) Subscribe() error {
	subCallback := func(services []model.SubscribeService, err error) {
		if len(services) > 0 {
			actives := float64(len(services))
			log.Printf("subscribe callback return services:%s \n\n", util.ToJsonString(services))
			var newFlowControlRules []FlowControlOption
			for _, fr := range a.rule.FlowControlRules {
				newFlowRule := fr
				newFlowRule.Threshold = fr.Threshold / actives
				newFlowControlRules = append(newFlowControlRules, newFlowRule)
			}
			log.Printf("balanced flow control:%s \n", util.ToJsonString(newFlowControlRules))
			a.loadFlowControlRules(newFlowControlRules...)
		}
	}
	subParam := &vo.SubscribeParam{
		ServiceName:       a.serviceName,
		GroupName:         a.group,
		SubscribeCallback: subCallback,
	}
	return a.nameClient.Subscribe(subParam)
}

//Deregister deregister service
func (a *Awarent) Deregister() (bool, error) {
	vo := vo.DeregisterInstanceParam{
		Ip:        util.LocalIP(),
		Port:      a.port,
		GroupName: a.group,
	}
	return a.nameClient.DeregisterInstance(vo)
}

//GetConfig get config from nacos with config dataid
func (a *Awarent) GetConfig(configID string) (string, error) {
	return a.configClient.GetConfig(vo.ConfigParam{
		DataId: configID,
		Group:  a.group,
	})
}

//ConfigOnChange listen on config change.
func (a *Awarent) ConfigOnChange(configID string, callback func(data string)) error {
	onChange := func(ns, group, dataId, data string) {
		fmt.Printf("config:%s changed, content:%s\n", configID, data)
		if callback != nil {
			callback(data)
		}
	}
	vo := vo.ConfigParam{
		Group:    a.group,
		DataId:   configID,
		OnChange: onChange,
	}
	return a.configClient.ListenConfig(vo)
}

//loadFlowControlRules load flow control rules
func (a *Awarent) loadFlowControlRules(rules ...FlowControlOption) (bool, error) {
	var sentinelRules []*flow.Rule
	for _, ruleItem := range rules {
		sentinelRule := &flow.Rule{
			Resource:               ruleItem.Resource,
			Threshold:              ruleItem.Threshold,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        flow.Reject,
			StatIntervalInMs:       1000,
		}
		sentinelRules = append(sentinelRules, sentinelRule)
	}
	return flow.LoadRules(sentinelRules)
}

// Metrics wrappers the standard http.Handler to gin.HandlerFunc
func (a *Awarent) Metrics() gin.HandlerFunc {
	searcher, err := metric.NewDefaultMetricSearcher(a.logDir, a.serviceName)
	if err != nil {
		return func(c *gin.Context) {
			c.AbortWithStatus(http.StatusInternalServerError)
		}
	}
	return func(c *gin.Context) {
		beginTimeMs := uint64((time.Now().Add(-2 * time.Second)).UnixNano() / 1e6)
		beginTimeMs = beginTimeMs - beginTimeMs%1000
		items, err := searcher.FindByTimeAndResource(beginTimeMs, beginTimeMs, "")
		if err != nil {
			c.String(http.StatusInternalServerError, "500 - Something bad")
			return
		}
		b := bytes.Buffer{}
		for _, item := range items {
			if len(item.Resource) == 0 {
				item.Resource = "__default__"
			}
			if fatStr, err := item.ToFatString(); err == nil {
				b.WriteString(fatStr)
				b.WriteByte('\n')
			}

		}
		c.String(http.StatusOK, b.String())
	}
}

//IPFilter ip filter with options
func (a *Awarent) IPFilter() gin.HandlerFunc {
	opts := a.rule.IPFilterRules
	ipfilter = New(opts)
	return func(c *gin.Context) {
		if ipfilter.urlPath == c.Request.URL.Path {
			param := c.Query(ipfilter.urlParam)
			blocked := false
			ip := c.ClientIP()
			if !ipfilter.Allowed(ip) {
				blocked = true
			} else if !ipfilter.Authorized(ip, param) {
				blocked = true
			}
			if blocked {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
		}
		c.Next()
	}
}

//Sentinel awarent gin use middleware
func (a *Awarent) Sentinel() gin.HandlerFunc {
	param := a.rule.ResourceParam
	endpoint := a.rule.IPFilterRules.URLPath
	return SentinelMiddleware(
		// speicify which url path working with sentinel
		endpoint,
		// customize resource extractor if required
		// method_path by default
		WithResourceExtractor(func(ctx *gin.Context) string {
			return ctx.Query(param)
		}),
		// customize block fallback if required
		// abort with status 429 by default
		WithBlockFallback(func(ctx *gin.Context) {
			ctx.AbortWithStatus(http.StatusTooManyRequests)
			// ctx.AbortWithStatusJSON(http.StatusTooManyRequests, map[string]interface{}{
			// 	"err":  "too many request; the quota used up",
			// 	"code": 10222,
			// })
		}),
	)
}
