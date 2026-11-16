package logger

import (
	"context"
	"sync"
)

// requestFields 是请求级字段累加器：任意层（auth 中间件、handler、service）
// 都可以往里加 key/value，由访问日志统一输出成一行，避免每个模块各打一条。
type requestFields struct {
	mu sync.Mutex
	kv []any
}

type requestFieldsKey struct{}

// WithRequestFields 给 ctx 挂载字段累加器；RequestID 中间件为每个请求调用一次。
func WithRequestFields(ctx context.Context) context.Context {
	return context.WithValue(ctx, requestFieldsKey{}, &requestFields{})
}

// Add 向请求级字段累加器追加 kv。没有累加器时（单元测试、定时任务）是 no-op。
// 奇数个参数时忽略末尾多余的 key；值为 nil 的键值对跳过。
func Add(ctx context.Context, kv ...any) {
	f, ok := ctx.Value(requestFieldsKey{}).(*requestFields)
	if !ok || f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == nil {
			continue
		}
		f.kv = append(f.kv, kv[i], kv[i+1])
	}
}

// HasRequestFields 判断 ctx 是否挂在请求内（有累加器）。
// 用于业务事件：请求内只注入字段（由访问日志输出一行），请求外直接输出事件行。
func HasRequestFields(ctx context.Context) bool {
	f, ok := ctx.Value(requestFieldsKey{}).(*requestFields)
	return ok && f != nil
}

// RequestFields 返回请求级字段的快照（key/value 交替，同 key 后者覆盖前者）。
func RequestFields(ctx context.Context) []any {
	f, ok := ctx.Value(requestFieldsKey{}).(*requestFields)
	if !ok || f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]any(nil), f.kv...)
}
