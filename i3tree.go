package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"
)

// i3Node is a minimal i3 tree node. The Window field is the X11 window ID,
// which the i3 library's Node struct also omits.
type i3Node struct {
	ID               int64            `json:"id"`
	Type             string           `json:"type"`
	Name             string           `json:"name"`
	Window           int64            `json:"window"`
	WindowProperties windowProperties `json:"window_properties"`
	Nodes            []*i3Node        `json:"nodes"`
	FloatingNodes    []*i3Node        `json:"floating_nodes"`
}

type windowProperties struct {
	Class string `json:"class"`
}

// findWorkspace walks the i3 tree and returns the workspace name for the first
// node matching predicate.
func findWorkspace(node *i3Node, ws string, predicate func(*i3Node) bool) string {
	if node.Type == "workspace" {
		ws = node.Name
	}

	if predicate(node) {
		return ws
	}

	for _, c := range node.Nodes {
		if result := findWorkspace(c, ws, predicate); result != "" {
			return result
		}
	}

	for _, c := range node.FloatingNodes {
		if result := findWorkspace(c, ws, predicate); result != "" {
			return result
		}
	}

	return ""
}

// getI3Tree fetches the i3 layout tree via raw IPC so that we can access the
// window ID field that the Go i3 library doesn't expose.
func getI3Tree() (*i3Node, error) {
	out, err := exec.Command("i3", "--get-socketpath").Output()
	if err != nil {
		return nil, fmt.Errorf("get i3 socketpath: %w", err)
	}

	path := strings.TrimSpace(string(out))

	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("connect to i3: %w", err)
	}

	defer func() {
		_ = conn.Close()
	}()

	// i3 IPC header: "i3-ipc" magic (6 bytes) + payload length (4 bytes) +
	// message type (4 bytes). GET_TREE = type 4, no payload.
	var header [14]byte

	copy(header[0:6], "i3-ipc")
	binary.NativeEndian.PutUint32(header[6:10], 0)
	binary.NativeEndian.PutUint32(header[10:14], 4)

	_, err = conn.Write(header[:])
	if err != nil {
		return nil, fmt.Errorf("send tree request: %w", err)
	}

	var respHeader [14]byte

	_, err = io.ReadFull(conn, respHeader[:])
	if err != nil {
		return nil, fmt.Errorf("read response header: %w", err)
	}

	length := binary.NativeEndian.Uint32(respHeader[6:10])

	payload := make([]byte, length)

	_, err = io.ReadFull(conn, payload)
	if err != nil {
		return nil, fmt.Errorf("read response payload: %w", err)
	}

	var root i3Node

	err = json.Unmarshal(payload, &root)
	if err != nil {
		return nil, fmt.Errorf("unmarshal tree: %w", err)
	}

	return &root, nil
}
