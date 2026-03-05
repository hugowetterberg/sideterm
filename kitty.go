package main

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

var kittyVersion = []int{0, 14, 2}

type kittyCommand struct {
	Cmd     string `json:"cmd"`
	Version []int  `json:"version"`
	NoResp  bool   `json:"no_response,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

type kittyResponse struct {
	OK   bool            `json:"ok"`
	Data json.RawMessage `json:"data"`
	Err  string          `json:"error"`
}

// Tab represents a kitty tab from the ls output.
type Tab struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// osWindow is the top-level structure returned by kitty ls.
type osWindow struct {
	Tabs []Tab `json:"tabs"`
}

func sendCommand(socketPath string, cmd string, payload any) (json.RawMessage, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("dial kitty socket: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	kc := kittyCommand{
		Cmd:     cmd,
		Version: kittyVersion,
		Payload: payload,
	}

	cmdJSON, err := json.Marshal(kc)
	if err != nil {
		return nil, fmt.Errorf("marshal command: %w", err)
	}

	// DCS frame: ESC P @kitty-cmd <JSON> ESC \
	frame := fmt.Sprintf("\x1bP@kitty-cmd%s\x1b\\", cmdJSON)

	_, err = conn.Write([]byte(frame))
	if err != nil {
		return nil, fmt.Errorf("write command: %w", err)
	}

	// Read response — also DCS framed.
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, readErr := conn.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		// Check if we have the full response (ends with ESC \).
		if len(buf) >= 2 && buf[len(buf)-2] == 0x1b && buf[len(buf)-1] == '\\' {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read response: %w", readErr)
		}
	}

	// Strip DCS framing.
	raw := string(buf)
	const prefix = "\x1bP@kitty-cmd"
	const suffix = "\x1b\\"
	if !strings.HasPrefix(raw, prefix) || !strings.HasSuffix(raw, suffix) {
		return nil, fmt.Errorf("unexpected response frame: %q", raw)
	}
	jsonData := raw[len(prefix) : len(raw)-len(suffix)]

	var resp kittyResponse
	err = json.Unmarshal([]byte(jsonData), &resp)
	if err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if !resp.OK {
		return nil, fmt.Errorf("kitty error: %s", resp.Err)
	}

	return resp.Data, nil
}

func listTabs(socketPath string) ([]Tab, error) {
	data, err := sendCommand(socketPath, "ls", nil)
	if err != nil {
		return nil, fmt.Errorf("list tabs: %w", err)
	}

	// The data field is a JSON-encoded string containing the actual JSON array.
	var lsJSON string
	err = json.Unmarshal(data, &lsJSON)
	if err != nil {
		return nil, fmt.Errorf("unwrap ls response: %w", err)
	}

	var osWindows []osWindow
	err = json.Unmarshal([]byte(lsJSON), &osWindows)
	if err != nil {
		return nil, fmt.Errorf("parse ls response: %w", err)
	}

	var tabs []Tab
	for _, ow := range osWindows {
		tabs = append(tabs, ow.Tabs...)
	}

	return tabs, nil
}

func createProjectTab(socketPath, projectName, projectPath string) error {
	// Create a new tab with the project name.
	_, err := sendCommand(socketPath, "launch", map[string]any{
		"type":      "tab",
		"tab_title": projectName,
		"cwd":       projectPath,
	})
	if err != nil {
		return fmt.Errorf("create tab: %w", err)
	}

	// Switch the new tab to the splits layout so hsplit works.
	_, err = sendCommand(socketPath, "goto-layout", map[string]any{
		"layout": "splits",
	})
	if err != nil {
		return fmt.Errorf("set layout: %w", err)
	}

	// Add a horizontal split in the newly focused tab.
	_, err = sendCommand(socketPath, "launch", map[string]any{
		"type":     "window",
		"location": "hsplit",
		"cwd":      projectPath,
	})
	if err != nil {
		return fmt.Errorf("create vsplit: %w", err)
	}

	return nil
}

func focusTab(socketPath string, tabID int) error {
	_, err := sendCommand(socketPath, "focus-tab", map[string]any{
		"match": fmt.Sprintf("id:%d", tabID),
	})
	if err != nil {
		return fmt.Errorf("focus tab: %w", err)
	}

	return nil
}
