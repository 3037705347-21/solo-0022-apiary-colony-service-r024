package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type client struct {
	base string
	http *http.Client
}

type response struct {
	status int
	body   []byte
	header http.Header
}

func newClient(base string) *client {
	return &client{base: base, http: &http.Client{Timeout: 5 * time.Second}}
}

func (c *client) call(method, path string, payload any) (*response, error) {
	return c.callWithHeaders(method, path, payload, nil)
}

func (c *client) callWithHeaders(method, path string, payload any, headers map[string]string) (*response, error) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, c.base+path, body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	reply, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer reply.Body.Close()
	raw, err := io.ReadAll(reply.Body)
	if err != nil {
		return nil, err
	}
	return &response{status: reply.StatusCode, body: raw, header: reply.Header}, nil
}

func (c *client) expect(method, path string, payload any, wantStatus int, wantFragments ...string) (*response, error) {
	return c.expectWithHeaders(method, path, payload, nil, wantStatus, wantFragments...)
}

func (c *client) expectWithHeaders(method, path string, payload any, headers map[string]string, wantStatus int, wantFragments ...string) (*response, error) {
	reply, err := c.callWithHeaders(method, path, payload, headers)
	if err != nil {
		return nil, err
	}
	if err := reply.mustStatus(method, path, wantStatus); err != nil {
		return nil, err
	}
	text := reply.text()
	for _, fragment := range wantFragments {
		if !strings.Contains(text, fragment) {
			return nil, fmt.Errorf("%s %s 响应缺少 %q：%s", method, path, fragment, text)
		}
	}
	return reply, nil
}

func (r *response) mustStatus(method, path string, wantStatus int) error {
	if r.status != wantStatus {
		return fmt.Errorf("%s %s 期望状态 %d，实际 %d，响应 %s", method, path, wantStatus, r.status, r.text())
	}
	return nil
}

func (r *response) text() string {
	return string(r.body)
}

func (r *response) count(substring string) int {
	return strings.Count(r.text(), substring)
}

func (r *response) mustNotContain(method, path, fragment string) error {
	if strings.Contains(r.text(), fragment) {
		return fmt.Errorf("%s %s 响应不应包含 %q：%s", method, path, fragment, r.text())
	}
	return nil
}

func (r *response) mustHeader(name, want string) error {
	if got := r.header.Get(name); got != want {
		return fmt.Errorf("响应头 %s 期望 %q，实际 %q", name, want, got)
	}
	return nil
}

func (r *response) mustOrder(before, after string) error {
	text := r.text()
	first := strings.Index(text, before)
	second := strings.Index(text, after)
	if first < 0 || second < 0 {
		return fmt.Errorf("响应缺少排序断言片段 %q / %q：%s", before, after, text)
	}
	if first >= second {
		return fmt.Errorf("响应顺序不符合预期，%q 应出现在 %q 之前：%s", before, after, text)
	}
	return nil
}
