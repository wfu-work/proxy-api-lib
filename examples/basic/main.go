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

	resp, err := client.Responses.Create(context.Background(), domains.ResponseRequest{
		Model: os.Getenv("OPENAI_MODEL"),
		Input: domains.InputText("Say hello in one short sentence."),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.OutputText())
}
