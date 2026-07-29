package domains

import "encoding/json"

// ResponseStream 是按顺序读取 Responses API 事件的流式迭代器。
type ResponseStream struct {
	next  func() bool
	event func() StreamEvent
	err   func() error
	close func() error
}

// NewResponseStream 根据底层回调创建流式迭代器。
// 该构造方法主要供协议实现层连接具体的 SSE 读取器。
func NewResponseStream(next func() bool, event func() StreamEvent, err func() error, close func() error) *ResponseStream {
	return &ResponseStream{
		next:  next,
		event: event,
		err:   err,
		close: close,
	}
}

// Next 将流推进到下一条事件；返回 false 表示结束或发生错误。
func (s *ResponseStream) Next() bool {
	if s == nil || s.next == nil {
		return false
	}
	return s.next()
}

// Event 返回最近一次 Next 成功读取的事件。
func (s *ResponseStream) Event() StreamEvent {
	if s == nil || s.event == nil {
		return StreamEvent{}
	}
	return s.event()
}

// Err 返回流读取过程中发生的错误；正常结束时返回 nil。
func (s *ResponseStream) Err() error {
	if s == nil || s.err == nil {
		return nil
	}
	return s.err()
}

// Close 主动关闭底层响应流；对空流调用时返回 nil。
func (s *ResponseStream) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	return s.close()
}

// StreamEvent 是流式接口使用的统一事件结构。
type StreamEvent struct {
	Type string
	Data json.RawMessage
}
