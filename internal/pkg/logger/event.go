package logger

import "context"

// Event 记录一条业务状态变更（如 payment.settled / order.canceled）。
//
// 在请求内调用时只把字段注入请求级累加器，由访问日志输出唯一一行
// （event 字段标记事件名），避免"访问日志 + 事件日志"重复；
// 在后台任务（定时任务、看门狗）中调用时直接输出一条事件日志。
func Event(ctx context.Context, event, msg string, kv ...any) {
	if HasRequestFields(ctx) {
		Add(ctx, "event", event)
		Add(ctx, kv...)
		return
	}
	FromCtx(ctx).Infow(msg, append([]any{"event", event}, kv...)...)
}

// WarnEvent 记录需要关注但可预期的异常（回调验签失败、退款异常等）。
// 事件行始终输出：这类信息不能只埋在一条 200 访问日志里。
func WarnEvent(ctx context.Context, event, msg string, kv ...any) {
	FromCtx(ctx).Warnw(msg, append([]any{"event", event}, kv...)...)
}

// ErrorEvent 记录需要人工介入的失败（金额不一致、回调处理失败等）。
func ErrorEvent(ctx context.Context, event, msg string, kv ...any) {
	FromCtx(ctx).Errorw(msg, append([]any{"event", event}, kv...)...)
}
