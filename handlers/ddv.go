package handlers

import (
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	outfile, _ := os.OpenFile("run.log", os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	log.SetOutput(outfile)
}

//GetDDV demon
func GetDDV(c *gin.Context) {
	r := rand.Intn(10)
	time.Sleep(time.Duration(r) * time.Millisecond)
	cid := c.Query("cid")
	log.Printf("ip:%s,cid=%s", cid, c.ClientIP())
	c.String(200, "%s", time.Now().Format(time.RFC3339))
}
