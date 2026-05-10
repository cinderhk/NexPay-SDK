package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

const (
	CodeOK            = 0
	CodeBadRequest    = 40000
	CodeUnauthorized  = 40100
	CodeNotFound      = 40400
	CodeInternalError = 50000
	CodeUpstreamError = 50200
)

func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Body{Code: CodeOK, Message: "ok", Data: data})
}

func Fail(c *gin.Context, httpStatus, code int, msg string) {
	c.AbortWithStatusJSON(httpStatus, Body{Code: code, Message: msg})
}

func BadRequest(c *gin.Context, msg string) {
	Fail(c, http.StatusBadRequest, CodeBadRequest, msg)
}

func Internal(c *gin.Context, msg string) {
	Fail(c, http.StatusInternalServerError, CodeInternalError, msg)
}

func Upstream(c *gin.Context, msg string) {
	Fail(c, http.StatusBadGateway, CodeUpstreamError, msg)
}
