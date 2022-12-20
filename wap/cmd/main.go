package main

import (
	"flag"

	"go-wifi/api"

	"github.com/gin-gonic/gin"
)

func main() {
	var bind string
	flag.StringVar(&bind, "b", ":8081", "the address/port to bind to")
	flag.Parse()

	r := gin.Default()
	api.SetupRoutes(r)
	r.Run(bind)
}
