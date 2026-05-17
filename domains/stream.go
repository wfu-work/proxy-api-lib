package domains

import "encoding/json"

// ResponseStream is the streaming iterator surface for Responses API events.
type ResponseStream struct {
	next  func() bool
	event func() StreamEvent
	err   func() error
	close func() error
}

// NewResponseStream constructs a stream iterator for provider implementations.
func NewResponseStream(next func() bool, event func() StreamEvent, err func() error, close func() error) *ResponseStream {
	return &ResponseStream{
		next:  next,
		event: event,
		err:   err,
		close: close,
	}
}

func (s *ResponseStream) Next() bool {
	if s == nil || s.next == nil {
		return false
	}
	return s.next()
}

func (s *ResponseStream) Event() StreamEvent {
	if s == nil || s.event == nil {
		return StreamEvent{}
	}
	return s.event()
}

func (s *ResponseStream) Err() error {
	if s == nil || s.err == nil {
		return nil
	}
	return s.err()
}

func (s *ResponseStream) Close() error {
	if s == nil || s.close == nil {
		return nil
	}
	return s.close()
}

// StreamEvent is the normalized event shape used by streaming support.
type StreamEvent struct {
	Type string
	Data json.RawMessage
}
