package models

import (
	"fmt"
	"net/http"
)

// AppError 应用错误类型
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// 预定义错误
var (
	ErrInvalidRequest   = &AppError{Code: http.StatusBadRequest, Message: "请求格式无效"}
	ErrPathForbidden    = &AppError{Code: http.StatusForbidden, Message: "路径验证失败"}
	ErrFileNotFound     = &AppError{Code: http.StatusNotFound, Message: "文件不存在"}
	ErrInternalError    = &AppError{Code: http.StatusInternalServerError, Message: "内部服务器错误"}
	ErrMethodNotAllowed = &AppError{Code: http.StatusMethodNotAllowed, Message: "方法不允许"}
)

// NewAppError 创建新的应用错误
func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// NewBadRequestError 创建400错误
func NewBadRequestError(message string, err error) *AppError {
	return NewAppError(http.StatusBadRequest, message, err)
}

// NewForbiddenError 创建403错误
func NewForbiddenError(message string, err error) *AppError {
	return NewAppError(http.StatusForbidden, message, err)
}

// NewNotFoundError 创建404错误
func NewNotFoundError(message string, err error) *AppError {
	return NewAppError(http.StatusNotFound, message, err)
}

// NewInternalError 创建500错误
func NewInternalError(message string, err error) *AppError {
	return NewAppError(http.StatusInternalServerError, message, err)
}