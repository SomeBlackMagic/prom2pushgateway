package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"text/template"
	"time"
)

var version string = "dev"
var revision string = "000000000000000000000000000000"

var lastSuccess atomic.Value

type customMetric struct {
	Name  string
	Value float64
}

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

func renderTemplateFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("metrics").Parse(string(data))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{
		"Env": envMap(),
	})
	return buf.String(), err
}

func envMap() map[string]string {
	m := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			m[pair[0]] = pair[1]
		}
	}
	return m
}

func readMetricsFile(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	rendered, err := renderTemplateFile(path)
	if err != nil {
		return nil, err
	}

	// basic validation / cleanup
	var buf bytes.Buffer
	scanner := bufio.NewScanner(strings.NewReader(rendered))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		buf.WriteString(line + "\n")
	}
	return buf.Bytes(), nil
}

func main() {
	fmt.Printf("Start prom2pushgateway version=%s revision=%s\n", version, revision)

	sourceURL := getenv("SOURCE_URL", "http://app:8080/metrics")
	pushURL := getenv("PUSHGATEWAY_URL", "http://pushgateway:9091/metrics/job/example")
	pushUser := getenv("PUSHGATEWAY_USER", "")
	pushPass := getenv("PUSHGATEWAY_PASS", "")
	metricsFile := getenv("CUSTOM_METRICS_FILE", "/etc/custom-metrics.txt")

	interval := getenvDuration("INTERVAL", 15*time.Second)
	scrapeTimeout := getenvDuration("SCRAPE_TIMEOUT", 5*time.Second)
	pushTimeout := getenvDuration("PUSH_TIMEOUT", 5*time.Second)
	healthAddr := getenv("HEALTH_ADDR", ":8081")

	client := &http.Client{}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Health endpoint
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			ok, _ := lastSuccess.Load().(bool)
			if ok {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("not ready"))
			}
		})
		srv := &http.Server{Addr: healthAddr, Handler: mux}

		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			srv.Shutdown(shutdownCtx)
		}()

		fmt.Println("health endpoint listening on", healthAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Println("health server error:", err)
		}
	}()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("shutdown requested, exiting gracefully...")
			return
		default:
			customMetrics, err := readMetricsFile(metricsFile)
			if err != nil {
				if !os.IsNotExist(err) {
					fmt.Println("read metrics file:", err)
				}
			}
			run(client, sourceURL, pushURL, pushUser, pushPass, scrapeTimeout, pushTimeout, customMetrics)
			select {
			case <-ticker.C:
				continue
			case <-ctx.Done():
				fmt.Println("shutdown during wait, exiting gracefully...")
				return
			}
		}
	}
}

func run(client *http.Client, source, push, user, pass string, scrapeTimeout, pushTimeout time.Duration, customMetrics []byte) {
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

	// Append rendered template metrics
	if len(customMetrics) > 0 {
		body = append(body, []byte("\n")...)
		body = append(body, customMetrics...)
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

	fmt.Printf("[%s] pushed → %s (auth=%v, template metrics=%d bytes)\n",
		time.Now().Format(time.RFC3339),
		push,
		user != "",
		len(customMetrics))
}
