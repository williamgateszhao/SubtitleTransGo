package main

import (
	"os"
	"time"

	translator "github.com/Conight/go-googletrans"
)

func translateViaGoogle(text string) (string, error) {
	c := translator.Config{
		// Get the proxy from the environment variable
		Proxy: os.Getenv("http_proxy"),
	}
	t := translator.New(c)
	// Translate from auto-detected language to Simplified Chinese
	result, err := t.Translate(text, "auto", "zh-cn")
	if err != nil {
		// Retry logic: If the initial translation fails, wait for 30 seconds and try once more
		time.Sleep(30 * time.Second)
		result, err = t.Translate(text, "auto", "zh-cn") // retry once
		if err != nil {
			return "", err
		}
	}
	return result.Text, nil
}
