package response

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/triton-io/triton/pkg/setting"
)

type Response struct {
	Code setting.ResponseCode `json:"code"`
	Data interface{}          `json:"data"`
	Msg  string               `json:"msg"`
}

type PagedResponse struct {
	Response

	Start      int `json:"start"`
	PageSize   int `json:"page_size"`
	TotalCount int `json:"total_count"`
}

func failedWithStatusAndMessage(status int, msg string, c *gin.Context) {
	c.JSON(status, Response{
		Code: setting.Error,
		Data: struct{}{},
		Msg:  msg,
	})
}

func successWithStatus(status int, data interface{}, msg string, c *gin.Context) {
	c.JSON(status, Response{
		Code: setting.Success,
		Data: data,
		Msg:  msg,
	})
}

func PagedSuccess(data interface{}, start, size, totalCount int, c *gin.Context) {
	c.JSON(http.StatusOK, PagedResponse{
		Response: Response{
			Code: setting.Success,
			Data: data,
			Msg:  "",
		},
		Start:      start,
		PageSize:   size,
		TotalCount: totalCount,
	})
}

func Deleted(c *gin.Context) {
	successWithStatus(http.StatusNoContent, struct{}{}, "", c)
}

func Created(data interface{}, c *gin.Context) {
	successWithStatus(http.StatusCreated, data, "success", c)
}

func NotFound(c *gin.Context) {
	failedWithStatusAndMessage(http.StatusNotFound, "not found", c)
}

func AlreadyExists(c *gin.Context) {
	failedWithStatusAndMessage(http.StatusConflict, "already exists", c)
}

func ConflictWithMessage(msg string, c *gin.Context) {
	failedWithStatusAndMessage(http.StatusConflict, msg, c)
}

func ServerErrorWithErrorAndMessage(err error, msg string, c *gin.Context) {
	failedWithStatusAndMessage(http.StatusInternalServerError, fmt.Sprintf("%s: %v", msg, err), c)
}

func ServerErrorWithMessage(msg string, c *gin.Context) {
	failedWithStatusAndMessage(http.StatusInternalServerError, msg, c)
}

func BadRequestWithMessage(msg string, c *gin.Context) {
	failedWithStatusAndMessage(http.StatusBadRequest, msg, c)
}

func OkDetailed(data interface{}, message string, c *gin.Context) {
	successWithStatus(http.StatusOK, data, message, c)
}
