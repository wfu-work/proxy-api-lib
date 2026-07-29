package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wfu-work/proxy-api-lib/openai"
	"github.com/wfu-work/proxy-api-lib/proxyapi"
)

// main 演示使用 OpenAI Platform API Key 发起流式 Responses 请求。
func main() {
	client := proxyapi.NewClient(
		proxyapi.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	stream, err := client.Responses.Stream(context.Background(), openai.ResponseRequest{
		Model: os.Getenv("OPENAI_MODEL"),
		Input: openai.InputText("Count from one to three."),
	})
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	acc := openai.NewStreamAccumulator()
	for stream.Next() {
		event := stream.Event()
		acc.Add(event)
		fmt.Print(event.TextDelta())
	}
	if err := stream.Err(); err != nil {
		panic(err)
	}
	fmt.Println()
}
