package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return def
}

func getenvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

func main() {
	sourceURL := getenv("SOURCE_URL", "http://app:8080/metrics")
	pushURL := getenv("PUSHGATEWAY_URL", "http://pushgateway:9091/metrics/job/example")
	pushUser := getenv("PUSHGATEWAY_USER", "")
	pushPass := getenv("PUSHGATEWAY_PASS", "")

	interval := getenvDuration("INTERVAL", 15*time.Second)
	scrapeTimeout := getenvDuration("SCRAPE_TIMEOUT", 5*time.Second)
	pushTimeout := getenvDuration("PUSH_TIMEOUT", 5*time.Second)
	customName := getenv("CUSTOM_METRIC_NAME", "")
	customValue := getenvFloat("CUSTOM_METRIC_VALUE", 0)

	client := &http.Client{}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		run(client, sourceURL, pushURL, pushUser, pushPass, scrapeTimeout, pushTimeout, customName, customValue)
		<-ticker.C
	}
}

func run(client *http.Client, source, push, user, pass string, scrapeTimeout, pushTimeout time.Duration, customName string, customValue float64) {
	ctx, cancel := context.WithTimeout(context.Background(), scrapeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", source, nil)
	if err != nil {
		fmt.Println("scrape: build request:", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("scrape:", err)
		return
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		fmt.Println("read:", err)
		return
	}

	if customName != "" {
		body = append(body, []byte(fmt.Sprintf("\n%s %f\n", customName, customValue))...)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), pushTimeout)
	defer cancel2()

	req2, err := http.NewRequestWithContext(ctx2, "POST", push, bytes.NewReader(body))
	if err != nil {
		fmt.Println("push: build request:", err)
		return
	}
	req2.Header.Set("Content-Type", "text/plain")

	if user != "" && pass != "" {
		token := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
		req2.Header.Set("Authorization", "Basic "+token)
	}

	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Println("push:", err)
		return
	}
	io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()

	fmt.Printf("[%s] pushed → %s (auth=%v custom=%s=%v)\n",
		time.Now().Format(time.RFC3339),
		push,
		user != "",
		customName,
		customValue)
}
