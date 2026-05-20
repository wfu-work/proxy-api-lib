package main

import (
	"context"
	"fmt"
	"os"

	proxyapi "github.com/wfu-work/proxy-api-lib"
	"github.com/wfu-work/proxy-api-lib/domains"
	"github.com/wfu-work/proxy-api-lib/openai"
)

func main() {
	client := proxyapi.NewClient(
		proxyapi.WithProvider(openai.New()),
		proxyapi.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
	)

	stream, err := client.Responses.Stream(context.Background(), domains.ResponseRequest{
		Model: os.Getenv("OPENAI_MODEL"),
		Input: domains.InputText("Count from one to three."),
	})
	if err != nil {
		panic(err)
	}
	defer stream.Close()

	acc := domains.NewStreamAccumulator()
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
