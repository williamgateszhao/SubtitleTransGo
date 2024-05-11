package main

import (
	"os"
	"time"

	translator "github.com/Conight/go-googletrans"
)

func translateViaGoogle(text string) (string, error) {
	c := translator.Config{
		Proxy: os.Getenv("http_proxy"), // Get the proxy from the environment variable
	}
	t := translator.New(c)
	result, err := t.Translate(text, "auto", "zh-cn") // Translate from auto-detected language to Simplified Chinese
	if err != nil {
		time.Sleep(30 * time.Second)
		result, err = t.Translate(text, "auto", "zh-cn") // retry once
		if err != nil {
			return "", err
		}
	}
	return result.Text, nil
}
