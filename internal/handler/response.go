package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`    // HTTP 状态码
	Data    interface{} `json:"data"`    // 响应数据
	Message string      `json:"message"` // 响应消息
}

// PageData 分页数据结构
type PageData struct {
	List  interface{} `json:"list"`  // 数据列表
	Total int64       `json:"total"` // 总数
	Page  int         `json:"page"`  // 当前页码
	Size  int         `json:"size"`  // 每页数量
}

// EmptyData 空数据结构（用于不返回数据的成功响应）
type EmptyData struct{}

// Success 成功响应（带数据）
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Data:    data,
		Message: "success",
	})
}

// SuccessWithMessage 成功响应（自定义消息）
func SuccessWithMessage(c *gin.Context, data interface{}, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Data:    data,
		Message: message,
	})
}

// SuccessWithEmpty 成功响应（无数据）
func SuccessWithEmpty(c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code:    http.StatusOK,
		Data:    EmptyData{},
		Message: "success",
	})
}

// Created 创建成功响应
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{
		Code:    http.StatusCreated,
		Data:    data,
		Message: "created",
	})
}

// SuccessWithPage 分页成功响应
func SuccessWithPage(c *gin.Context, list interface{}, total int64, page, size int) {
	c.JSON(http.StatusOK, Response{
		Code: http.StatusOK,
		Data: PageData{
			List:  list,
			Total: total,
			Page:  page,
			Size:  size,
		},
		Message: "success",
	})
}

// Error 错误响应（通用）
func Error(c *gin.Context, code int, message string) {
	c.JSON(code, Response{
		Code:    code,
		Data:    EmptyData{},
		Message: message,
	})
}

// BadRequest 400 错误
func BadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, Response{
		Code:    http.StatusBadRequest,
		Data:    EmptyData{},
		Message: message,
	})
}

// Unauthorized 401 错误
func Unauthorized(c *gin.Context, message string) {
	c.JSON(http.StatusUnauthorized, Response{
		Code:    http.StatusUnauthorized,
		Data:    EmptyData{},
		Message: message,
	})
}

// Forbidden 403 错误
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, Response{
		Code:    http.StatusForbidden,
		Data:    EmptyData{},
		Message: message,
	})
}

// NotFound 404 错误
func NotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, Response{
		Code:    http.StatusNotFound,
		Data:    EmptyData{},
		Message: message,
	})
}

// NotImplemented 501 错误
func NotImplemented(c *gin.Context, message string) {
	c.JSON(http.StatusNotImplemented, Response{
		Code:    http.StatusNotImplemented,
		Data:    EmptyData{},
		Message: message,
	})
}

// InternalServerError 500 错误
func InternalServerError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, Response{
		Code:    http.StatusInternalServerError,
		Data:    EmptyData{},
		Message: message,
	})
}

// ServiceUnavailable 503 错误
func ServiceUnavailable(c *gin.Context, message string) {
	c.JSON(http.StatusServiceUnavailable, Response{
		Code:    http.StatusServiceUnavailable,
		Data:    EmptyData{},
		Message: message,
	})
}

// ErrorWithData 错误响应（带数据）
func ErrorWithData(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, Response{
		Code:    code,
		Data:    data,
		Message: message,
	})
}

// ============================================================================
// 常用错误消息常量
// ============================================================================

const (
	// 通用错误
	ErrInvalidParams     = "参数错误"
	ErrInternalServer    = "服务器内部错误"
	ErrUnauthorized      = "未授权"
	ErrForbidden         = "无权限"
	ErrNotFound          = "资源不存在"
	ErrServiceUnavailable = "服务暂时不可用"

	// 用户相关
	ErrUserNotFound      = "用户不存在"
	ErrUserAlreadyExists = "用户已存在"
	ErrInvalidUser       = "用户信息无效"

	// 账号相关
	ErrAccountNotFound     = "账号不存在"
	ErrAccountAlreadyBound = "账号已绑定"
	ErrAccountBindFailed   = "账号绑定失败"
	ErrAccountUnbindFailed = "账号解绑失败"

	// Token 相关
	ErrInvalidToken       = "无效的令牌"
	ErrExpiredToken       = "令牌已过期"
	ErrRefreshTokenFailed = "刷新令牌失败"

	// 二维码相关
	ErrQRCodeGenFailed = "生成二维码失败"
	ErrQRCodeExpired   = "二维码已过期"
	ErrQRCodeInvalid   = "无效的二维码"

	// 视频相关
	ErrVideoNotFound     = "视频不存在"
	ErrVideoCreateFailed = "创建视频失败"
	ErrVideoUpdateFailed = "更新视频失败"
	ErrVideoDeleteFailed = "删除视频失败"

	// 数据库相关
	ErrDatabaseQuery  = "数据库查询失败"
	ErrDatabaseCreate = "数据库创建失败"
	ErrDatabaseUpdate = "数据库更新失败"
	ErrDatabaseDelete = "数据库删除失败"
)

// ============================================================================
// 成功消息常量
// ============================================================================

const (
	MsgSuccess         = "success"
	MsgCreated         = "创建成功"
	MsgUpdated         = "更新成功"
	MsgDeleted         = "删除成功"
	MsgBound           = "绑定成功"
	MsgUnbound         = "解绑成功"
	MsgRefreshed       = "刷新成功"
	MsgOperationSuccess = "操作成功"
)
