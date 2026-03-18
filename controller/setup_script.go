package controller

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed scripts/setup.sh
var setupSh []byte

//go:embed scripts/setup.ps1
var setupPs1 []byte

func GetSetupSh(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", setupSh)
}

func GetSetupPs1(c *gin.Context) {
	c.Data(http.StatusOK, "text/plain; charset=utf-8", setupPs1)
}
