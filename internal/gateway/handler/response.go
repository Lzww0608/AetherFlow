package handler

import (
	"encoding/json"
	"net/http"
)

// Response 通用响应结构
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

// SuccessResponse 成功响应
func SuccessResponse(w http.ResponseWriter, data interface{}, requestID string) {
	response := Response{
		Code:      0,
		Message:   "success",
		Data:      data,
		RequestID: requestID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// ErrorResponse 错误响应
func ErrorResponse(w http.ResponseWriter, statusCode int, message string, requestID string) {
	response := Response{
		Code:      statusCode,
		Message:   message,
		RequestID: requestID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// BadRequestResponse 400错误
func BadRequestResponse(w http.ResponseWriter, message string, requestID string) {
	ErrorResponse(w, http.StatusBadRequest, message, requestID)
}

// UnauthorizedResponse 401错误
func UnauthorizedResponse(w http.ResponseWriter, message string, requestID string) {
	ErrorResponse(w, http.StatusUnauthorized, message, requestID)
}

// ForbiddenResponse 403错误
func ForbiddenResponse(w http.ResponseWriter, message string, requestID string) {
	ErrorResponse(w, http.StatusForbidden, message, requestID)
}

// NotFoundResponse 404错误
func NotFoundResponse(w http.ResponseWriter, message string, requestID string) {
	ErrorResponse(w, http.StatusNotFound, message, requestID)
}

// InternalServerErrorResponse 500错误
func InternalServerErrorResponse(w http.ResponseWriter, message string, requestID string) {
	ErrorResponse(w, http.StatusInternalServerError, message, requestID)
}
