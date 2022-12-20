package api

import (
	"fmt"
	"net/http"
	"os/exec"
	"time"

	wireless "go-wifi"

	"github.com/gin-gonic/gin"
)

type ApConnectInfo struct {
	Password string `json:"password"`
	SSID     string `json:"ssid"`
}

func json(err error) map[string]string {
	return map[string]string{"error": err.Error()}
}

func errStatus(err error) int {
	switch err {
	case wireless.ErrFailBusy:
		return 409
	case wireless.ErrNoIdentifier, wireless.ErrAssocRejected, wireless.ErrSSIDNotFound:
		return 400
	default:
		return 500
	}
}

func listInterfaces(c *gin.Context) {
	c.JSON(200, wireless.Interfaces())
}

func connectAp(c *gin.Context) {
	var body ApConnectInfo
	err := c.BindJSON(&body)

	if err != nil {
		c.JSON(http.StatusBadRequest, "bad request")
	}
	fmt.Println(body)

	if len(body.Password) > 0 && len(body.SSID) > 0 {
		_, err := exec.Command("nmcli", "d", "wifi", "connect", body.SSID, "password", body.Password).Output()
		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusBadRequest, "ssid or password is incorrect")
		}
	} else if len(body.SSID) > 0 {
		_, err := exec.Command("nmcli", "d", "wifi", "connect", body.SSID).Output()
		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusBadRequest, "ssid is incorrect")
		}
	}
}

func listAccesPoints(c *gin.Context) {
	iface := c.Param("iface")

	wc, err := wireless.NewClient(iface)
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(errStatus(err), json(err))
		return
	}
	defer wc.Close()

	wc.ScanTimeout = 3 * time.Second

	aps, err := wc.Scan()
	if err != nil {
		c.Error(err)
		c.AbortWithStatusJSON(errStatus(err), json(err))
		return
	}

	c.JSON(200, aps)
}
