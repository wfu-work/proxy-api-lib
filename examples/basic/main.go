package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wfu-work/proxy-api-lib/openai"
	"github.com/wfu-work/proxy-api-lib/proxyapi"
)

// main 演示使用 OpenAI Platform API Key 发起非流式 Responses 请求。
func main() {
	client := proxyapi.NewClient(
		proxyapi.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	resp, err := client.Responses.Create(context.Background(), openai.ResponseRequest{
		Model: os.Getenv("OPENAI_MODEL"),
		Input: openai.InputText("Say hello in one short sentence."),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.OutputText())
}
