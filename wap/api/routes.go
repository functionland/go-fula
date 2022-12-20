package api

import (
	"github.com/gin-gonic/gin"
)

// SetupRoutes will put handlers to routes on the given router
func SetupRoutes(r gin.IRouter) {
	r.GET("/interfaces", listInterfaces)
	r.POST("/interfaces/:iface/connect", connectAp)

	r.GET("/interfaces/:iface/aps", listAccesPoints)
}
