package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"incus-web/config"
)

// Client Incus HTTP 客户端
type Client struct {
	// httpClient 用于普通 JSON 请求：60s 总超时（含读响应体）
	httpClient *http.Client
	// streamClient 用于流式响应（备份/镜像/文件下载）：不设总超时，
	// 只限制响应头等待时间，避免大文件在 60s 处被静默截断。
	streamClient *http.Client
	baseURL      string
}

// dialIncus 通过配置的 Unix Socket 连接 Incus
func dialIncus(ctx context.Context, network, addr string) (net.Conn, error) {
	// 每次连接都取一次路径：面板可能先于 incusd 启动，届时 socket 尚未生成，
	// 延迟重解析可以让面板在 incus 启动后自动恢复，无需重启。
	return net.Dial("unix", config.IncusSocket())
}

// newUnixTransport 返回连接 Incus Unix Socket 的 HTTP transport
func newUnixTransport() *http.Transport {
	return &http.Transport{DialContext: dialIncus}
}

// NewClient 创建 Incus 客户端，使用 Unix Socket 通信
func NewClient() *Client {
	transport := newUnixTransport()
	transport.MaxIdleConns = 10
	transport.IdleConnTimeout = 30 * time.Second

	streamTransport := newUnixTransport()
	streamTransport.MaxIdleConns = 10
	streamTransport.IdleConnTimeout = 30 * time.Second
	// 仅限制等待响应头的时间；响应体可长时间传输（大备份/大镜像）
	streamTransport.ResponseHeaderTimeout = 30 * time.Second

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
		streamClient: &http.Client{
			Transport: streamTransport,
			// 不设 Timeout：由调用方/客户端断开决定何时结束
		},
		baseURL: config.IncusBaseURL,
	}
}

// doRequest 通用 HTTP 请求方法
func (c *Client) doRequest(method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	return c.doRequestWith(c.httpClient, method, path, body, headers)
}

// doRequestWith 使用指定 http.Client 发起请求
func (c *Client) doRequestWith(httpClient *http.Client, method, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	url := c.baseURL + path

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("incus request failed: %w", err)
	}
	return resp, nil
}

// Get 发起 GET 请求并解析 JSON 响应到 result
func (c *Client) Get(path string, result interface{}) error {
	resp, err := c.doRequest("GET", path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("incus API error (status %d): %s", resp.StatusCode, string(body))
	}
	return json.NewDecoder(resp.Body).Decode(result)
}

// GetRaw 发起 GET 请求，返回原始响应体（流式，不设总超时）
func (c *Client) GetRaw(path string) (*http.Response, error) {
	resp, err := c.doRequestWith(c.streamClient, "GET", path, nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("incus API error (status %d): %s", resp.StatusCode, string(body))
	}
	return resp, nil
}

// Post 发起 POST 请求，body 为 JSON 序列化后的数据
func (c *Client) Post(path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	resp, err := c.doRequest("POST", path, bodyReader, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("incus API error (status %d): %s", resp.StatusCode, string(errBody))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// PostWithHeaders 发起带自定义头部的 POST 请求
func (c *Client) PostWithHeaders(path string, body io.Reader, headers map[string]string, result interface{}) error {
	resp, err := c.doRequest("POST", path, body, headers)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("incus API error (status %d): %s", resp.StatusCode, string(errBody))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// Put 发起 PUT 请求
func (c *Client) Put(path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	resp, err := c.doRequest("PUT", path, bodyReader, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("incus API error (status %d): %s", resp.StatusCode, string(errBody))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// Patch 发起 PATCH 请求
func (c *Client) Patch(path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body failed: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	resp, err := c.doRequest("PATCH", path, bodyReader, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("incus API error (status %d): %s", resp.StatusCode, string(errBody))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// Delete 发起 DELETE 请求
func (c *Client) Delete(path string, result interface{}) error {
	resp, err := c.doRequest("DELETE", path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("incus API error (status %d): %s", resp.StatusCode, string(errBody))
	}
	if result != nil {
		return json.NewDecoder(resp.Body).Decode(result)
	}
	return nil
}

// GetWebSocketURL 获取 WebSocket 的完整 URL（替换 http://unix 为 ws://unix）
func (c *Client) GetWebSocketURL(path string) string {
	return "http://unix" + path
}
