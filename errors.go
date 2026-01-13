package main

import "errors"

// 定义常用错误
var (
	ErrEmptyPath      = errors.New("路径不能为空")
	ErrPathNotAllowed = errors.New("路径不在允许的范围内")
	ErrInvalidRequest = errors.New("请求格式无效")
	ErrFileNotFound   = errors.New("文件不存在")
	ErrPermissionDenied = errors.New("权限不足")
)