package domains

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Responses API 中受支持的核心流事件类型。
const (
	EventResponseCompleted              = "response.completed"
	EventResponseFailed                 = "response.failed"
	EventResponseOutputItemAdded        = "response.output_item.added"
	EventResponseOutputTextDelta        = "response.output_text.delta"
	EventResponseFunctionArgumentsDelta = "response.function_call_arguments.delta"
	EventResponseFunctionArgumentsDone  = "response.function_call_arguments.done"
)

// TextDelta 从 Responses 流事件中提取文本增量。
func (e StreamEvent) TextDelta() string {
	payload, ok := e.OutputTextDelta()
	if !ok {
		return ""
	}
	return payload.Delta
}

// OutputTextDelta 解析 response.output_text.delta 事件。
func (e StreamEvent) OutputTextDelta() (OutputTextDeltaEvent, bool) {
	var payload OutputTextDeltaEvent
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		return payload, false
	}
	if payload.Type == "" {
		payload.Type = e.Type
	}
	if payload.Type != EventResponseOutputTextDelta {
		return payload, false
	}
	return payload, payload.Delta != ""
}

// FunctionCallArgumentsDelta 解析函数调用参数增量事件。
func (e StreamEvent) FunctionCallArgumentsDelta() (FunctionCallArgumentsDeltaEvent, bool) {
	var payload FunctionCallArgumentsDeltaEvent
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		return payload, false
	}
	if payload.Type == "" {
		payload.Type = e.Type
	}
	if payload.Type != EventResponseFunctionArgumentsDelta {
		return payload, false
	}
	return payload, payload.Delta != ""
}

// OutputItemAdded 解析 response.output_item.added 事件。
func (e StreamEvent) OutputItemAdded() (OutputItemAddedEvent, bool) {
	var payload OutputItemAddedEvent
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		return payload, false
	}
	if payload.Type == "" {
		payload.Type = e.Type
	}
	return payload, payload.Item.Type != ""
}

// CompletedResponse 从 response.completed 事件中解析最终响应。
func (e StreamEvent) CompletedResponse() (*Response, bool) {
	var payload struct {
		Type     string   `json:"type"`
		Response Response `json:"response"`
	}
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		return nil, false
	}
	if payload.Type == "" {
		payload.Type = e.Type
	}
	if payload.Type != EventResponseCompleted || payload.Response.ID == "" {
		return nil, false
	}
	return &payload.Response, true
}

// OutputTextDeltaEvent 描述文本输出增量事件。
type OutputTextDeltaEvent struct {
	Type         string `json:"type,omitempty"`
	ItemID       string `json:"item_id,omitempty"`
	OutputIndex  int    `json:"output_index,omitempty"`
	ContentIndex int    `json:"content_index,omitempty"`
	Delta        string `json:"delta,omitempty"`
	Text         string `json:"text,omitempty"`
}

// UnmarshalJSON 同时兼容 delta 和 text 字段，并统一写入 Delta。
func (e *OutputTextDeltaEvent) UnmarshalJSON(data []byte) error {
	type alias OutputTextDeltaEvent
	var out alias
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	if out.Delta == "" && out.Text != "" {
		out.Delta = out.Text
	}
	*e = OutputTextDeltaEvent(out)
	return nil
}

// FunctionCallArgumentsDeltaEvent 描述函数调用参数增量。
type FunctionCallArgumentsDeltaEvent struct {
	Type        string `json:"type,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
	CallID      string `json:"call_id,omitempty"`
	OutputIndex int    `json:"output_index,omitempty"`
	Delta       string `json:"delta,omitempty"`
}

// OutputItemAddedEvent 携带新加入响应流的输出项。
type OutputItemAddedEvent struct {
	Type        string       `json:"type,omitempty"`
	OutputIndex int          `json:"output_index,omitempty"`
	Item        ResponseItem `json:"item,omitempty"`
}

// StreamAccumulator 将 Responses 流聚合为文本、工具调用和最终响应。
type StreamAccumulator struct {
	text      strings.Builder
	calls     map[string]*ToolCall
	callOrder []string
	completed *Response
}

// NewStreamAccumulator 创建空的流事件聚合器。
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{calls: map[string]*ToolCall{}}
}

// Add 将一条流事件合并到当前聚合结果。
func (a *StreamAccumulator) Add(event StreamEvent) {
	if a.calls == nil {
		a.calls = map[string]*ToolCall{}
	}
	if delta, ok := event.OutputTextDelta(); ok {
		a.text.WriteString(delta.Delta)
		return
	}
	if added, ok := event.OutputItemAdded(); ok && added.Item.Type == "function_call" {
		call := ToolCall{
			ID:          added.Item.ID,
			CallID:      added.Item.CallID,
			Name:        added.Item.Name,
			Arguments:   added.Item.Arguments,
			OutputIndex: added.OutputIndex,
		}
		a.storeCall(callKey(call.ID, call.CallID, call.OutputIndex), call)
		return
	}
	if delta, ok := event.FunctionCallArgumentsDelta(); ok {
		key := callKey(delta.ItemID, delta.CallID, delta.OutputIndex)
		call := a.ensureCall(key, delta.ItemID, delta.CallID, delta.OutputIndex)
		call.Arguments += delta.Delta
		return
	}
	if response, ok := event.CompletedResponse(); ok {
		a.completed = response
	}
}

// Text 返回按事件顺序拼接后的文本。
func (a *StreamAccumulator) Text() string {
	if a == nil {
		return ""
	}
	return a.text.String()
}

// ToolCalls 按首次出现顺序返回聚合后的函数调用。
func (a *StreamAccumulator) ToolCalls() []ToolCall {
	if a == nil {
		return nil
	}
	calls := make([]ToolCall, 0, len(a.callOrder))
	for _, key := range a.callOrder {
		if call := a.calls[key]; call != nil {
			calls = append(calls, *call)
		}
	}
	return calls
}

// CompletedResponse 返回 response.completed 事件携带的最终响应。
func (a *StreamAccumulator) CompletedResponse() *Response {
	if a == nil {
		return nil
	}
	return a.completed
}

// storeCall 保存或更新函数调用，并维持首次出现的顺序。
func (a *StreamAccumulator) storeCall(key string, call ToolCall) {
	if existing := a.calls[key]; existing != nil {
		if call.ID != "" {
			existing.ID = call.ID
		}
		if call.CallID != "" {
			existing.CallID = call.CallID
		}
		if call.Name != "" {
			existing.Name = call.Name
		}
		if call.Arguments != "" {
			existing.Arguments = call.Arguments
		}
		return
	}
	a.calls[key] = &call
	a.callOrder = append(a.callOrder, key)
}

// ensureCall 获取已有函数调用；不存在时创建一个用于接收后续参数增量。
func (a *StreamAccumulator) ensureCall(key, itemID, callID string, outputIndex int) *ToolCall {
	if call := a.calls[key]; call != nil {
		return call
	}
	call := ToolCall{
		ID:          itemID,
		CallID:      callID,
		OutputIndex: outputIndex,
	}
	a.calls[key] = &call
	a.callOrder = append(a.callOrder, key)
	return &call
}

// callKey 根据可用标识生成稳定的函数调用聚合键。
func callKey(itemID, callID string, outputIndex int) string {
	if itemID != "" {
		return "item:" + itemID
	}
	if callID != "" {
		return "call:" + callID
	}
	return "index:" + strconv.Itoa(outputIndex)
}
