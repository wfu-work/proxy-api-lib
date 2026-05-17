package domains

import (
	"encoding/json"
	"strconv"
	"strings"
)

const (
	EventResponseCompleted              = "response.completed"
	EventResponseFailed                 = "response.failed"
	EventResponseOutputItemAdded        = "response.output_item.added"
	EventResponseOutputTextDelta        = "response.output_text.delta"
	EventResponseFunctionArgumentsDelta = "response.function_call_arguments.delta"
	EventResponseFunctionArgumentsDone  = "response.function_call_arguments.done"
)

// TextDelta extracts a common text delta field from Responses stream events.
func (e StreamEvent) TextDelta() string {
	payload, ok := e.OutputTextDelta()
	if !ok {
		return ""
	}
	return payload.Delta
}

// OutputTextDelta parses response.output_text.delta-style events.
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

// FunctionCallArgumentsDelta parses function call argument delta events.
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

// OutputItemAdded parses response.output_item.added events.
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

// CompletedResponse parses response.completed events.
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

// OutputTextDeltaEvent is the common payload for text deltas.
type OutputTextDeltaEvent struct {
	Type         string `json:"type,omitempty"`
	ItemID       string `json:"item_id,omitempty"`
	OutputIndex  int    `json:"output_index,omitempty"`
	ContentIndex int    `json:"content_index,omitempty"`
	Delta        string `json:"delta,omitempty"`
	Text         string `json:"text,omitempty"`
}

// UnmarshalJSON accepts both delta and text fields as text increments.
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

// FunctionCallArgumentsDeltaEvent is a function-call arguments delta.
type FunctionCallArgumentsDeltaEvent struct {
	Type        string `json:"type,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
	CallID      string `json:"call_id,omitempty"`
	OutputIndex int    `json:"output_index,omitempty"`
	Delta       string `json:"delta,omitempty"`
}

// OutputItemAddedEvent carries a new output item.
type OutputItemAddedEvent struct {
	Type        string       `json:"type,omitempty"`
	OutputIndex int          `json:"output_index,omitempty"`
	Item        ResponseItem `json:"item,omitempty"`
}

// StreamAccumulator aggregates a Responses stream into text and tool calls.
type StreamAccumulator struct {
	text      strings.Builder
	calls     map[string]*ToolCall
	callOrder []string
	completed *Response
}

// NewStreamAccumulator creates an empty stream accumulator.
func NewStreamAccumulator() *StreamAccumulator {
	return &StreamAccumulator{calls: map[string]*ToolCall{}}
}

// Add folds a stream event into the accumulator.
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

// Text returns the aggregated text deltas.
func (a *StreamAccumulator) Text() string {
	if a == nil {
		return ""
	}
	return a.text.String()
}

// ToolCalls returns aggregated function calls in first-seen order.
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

// CompletedResponse returns the final response from a response.completed event.
func (a *StreamAccumulator) CompletedResponse() *Response {
	if a == nil {
		return nil
	}
	return a.completed
}

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

func callKey(itemID, callID string, outputIndex int) string {
	if itemID != "" {
		return "item:" + itemID
	}
	if callID != "" {
		return "call:" + callID
	}
	return "index:" + strconv.Itoa(outputIndex)
}
