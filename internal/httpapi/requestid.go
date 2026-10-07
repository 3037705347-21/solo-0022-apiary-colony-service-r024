package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"sync/atomic"
	"time"

	"example.com/apiary-colony-service/internal/domain"
)

const (
	// RequestIDHeader 是调用方为一次写入提供请求编号的请求头；
	// 缺省时服务生成编号，并在同名响应头中回传，供调用方与审计事件核对。
	RequestIDHeader = "X-Request-Id"
)

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type requestIDContextKey struct{}

// withRequestID 为每个请求解析或生成请求编号，并通过响应头回传。
// 审计事件携带该编号，使事件可以回答“由哪次请求产生”。
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if requestID != "" {
			if len(requestID) < 1 || len(requestID) > 64 || !requestIDPattern.MatchString(requestID) {
				writeError(w, domain.Invalid("requestIdHeader",
					"%s 只能包含字母、数字、连字符与下划线，且长度不超过 64", RequestIDHeader))
				return
			}
		} else {
			requestID = generateRequestID()
		}
		w.Header().Set(RequestIDHeader, requestID)
		ctx := context.WithValue(r.Context(), requestIDContextKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requestIDFrom(r *http.Request) string {
	requestID, _ := r.Context().Value(requestIDContextKey{}).(string)
	return requestID
}

// generateRequestID 生成形如 req-xxxxxxxxxxxxxxxx 的服务端请求编号。
var requestIDFallbackSeq uint64

func generateRequestID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		// crypto/rand 失败属于运行环境异常，退回基于时间与计数器的编号避免重复。
		sequence := atomic.AddUint64(&requestIDFallbackSeq, 1)
		return "req-" + time.Now().UTC().Format("20060102T150405.000000000") + "-" +
			hex.EncodeToString([]byte{byte(sequence), byte(sequence >> 8)})
	}
	return "req-" + hex.EncodeToString(raw[:])
}
