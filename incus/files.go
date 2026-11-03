package incus

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"incus-web/model"
)

// fileHTTPClient 实例文件读写共用客户端：复用连接，避免每个目录条目都新建 Transport；
// 不设总超时（大文件下载/上传），仅限制等待响应头的时间。
var fileHTTPClient = func() *http.Client {
	t := newUnixTransport()
	t.MaxIdleConns = 20
	t.IdleConnTimeout = 30 * time.Second
	t.ResponseHeaderTimeout = 30 * time.Second
	return &http.Client{Transport: t}
}()

// ExecLS 使用 Incus files API 列出目录内容
func ExecLS(instanceName, dirPath string) ([]model.FileInfo, error) {
	client := fileHTTPClient

	if dirPath == "" {
		dirPath = "/"
	}

	url := "http://unix/1.0/instances/" + seg(instanceName) + "/files?path=" + url.QueryEscape(dirPath)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("X-Incus-Type", "directory")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list files error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list files error (status %d): %s", resp.StatusCode, string(body))
	}

	var incusResp struct {
		Metadata []string `json:"metadata"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&incusResp); err != nil {
		return nil, fmt.Errorf("parse file list error: %w", err)
	}

	// Incus 的目录列表响应头只描述被列目录本身，条目元数据需逐个 stat；
	// 并发发起以保持毫秒级延迟。
	files := make([]model.FileInfo, len(incusResp.Metadata))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, name := range incusResp.Metadata {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fullPath := dirPath
			if !strings.HasSuffix(fullPath, "/") {
				fullPath += "/"
			}
			fullPath += name

			fType, mode, uid, gid, size, modified := "file", "", 0, 0, int64(0), ""
			if st, ok := statEntry(instanceName, fullPath); ok {
				fType, mode, uid, gid, size, modified = st.fType, st.mode, st.uid, st.gid, st.size, st.modified
			}

			files[i] = model.FileInfo{
				Name:     name,
				Path:     fullPath,
				Type:     fType,
				Mode:     mode,
				UID:      uid,
				GID:      gid,
				Size:     size,
				Modified: modified,
			}
		}(i, name)
	}
	wg.Wait()
	return files, nil
}

// entryStat 单个文件条目的元数据
type entryStat struct {
	fType, mode, modified string
	uid, gid              int
	size                  int64
}

// statEntry 通过 files GET 的响应头获取单个条目的元数据
func statEntry(instanceName, filePath string) (entryStat, bool) {
	client := fileHTTPClient

	u := "http://unix/1.0/instances/" + seg(instanceName) + "/files?path=" + url.QueryEscape(filePath)
	req, _ := http.NewRequest("GET", u, nil)
	resp, err := client.Do(req)
	if err != nil {
		return entryStat{}, false
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 64))

	h := resp.Header
	get := func(k string) string {
		v := h.Values(k)
		if len(v) > 0 {
			return v[0]
		}
		return ""
	}
	var uid, gid int
	fmt.Sscanf(get("X-Incus-Uid"), "%d", &uid)
	fmt.Sscanf(get("X-Incus-Gid"), "%d", &gid)
	var size int64
	fmt.Sscanf(get("Content-Length"), "%d", &size)
	if size < 0 {
		size = 0
	}
	return entryStat{
		fType:    get("X-Incus-Type"),
		mode:     get("X-Incus-Mode"),
		uid:      uid,
		gid:      gid,
		size:     size,
		modified: get("X-Incus-Modified"),
	}, true
}

// FilePull 从实例中下载文件
func FilePull(instanceName, filePath string) (*http.Response, error) {
	client := fileHTTPClient

	url := "http://unix/1.0/instances/" + seg(instanceName) + "/files?path=" + url.QueryEscape(filePath)
	req, _ := http.NewRequest("GET", url, nil)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("file pull error (status %d): %s", resp.StatusCode, string(body))
	}
	return resp, nil
}

// FilePush 上传文件到实例
func FilePush(instanceName, remotePath string, file multipart.File) error {
	client := fileHTTPClient

	url := "http://unix/1.0/instances/" + seg(instanceName) + "/files?path=" + url.QueryEscape(remotePath)
	req, _ := http.NewRequest("POST", url, file)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Incus-Type", "file")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("file push error (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// FileDelete 删除实例中的文件
func FileDelete(instanceName, remotePath string) error {
	client := fileHTTPClient

	url := "http://unix/1.0/instances/" + seg(instanceName) + "/files?path=" + url.QueryEscape(remotePath)
	req, _ := http.NewRequest("DELETE", url, nil)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("file delete error (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetLogs 获取实例控制台日志。
// 容器日志为 lxc.log，虚拟机为 qemu.log；按顺序回退，避免 VM 的日志页直接报 404。
func (c *Client) GetLogs(instanceName string, lines int) (string, error) {
	var lastErr error
	for _, logName := range []string{"lxc.log", "qemu.log"} {
		url := "/instances/" + seg(instanceName) + "/logs/" + logName
		if lines > 0 {
			url += fmt.Sprintf("?lines=%d", lines)
		}
		resp, err := c.GetRaw(url)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		return string(body), nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("未找到实例日志")
	}
	return "", lastErr
}
