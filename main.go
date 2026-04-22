package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"go.i3wm.org/i3/v4"
)

var titlePattern = regexp.MustCompile(`^(.+) - (.+)$`)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("get home dir: %v", err)
	}

	socketPath := filepath.Join(home, "tmp", "emacs-kitty")

	err = startKitty(socketPath)
	if err != nil {
		log.Fatalf("start kitty: %v", err)
	}

	recv := i3.Subscribe(i3.WindowEventType)

	for recv.Next() {
		ev, ok := recv.Event().(*i3.WindowEvent)
		if !ok {
			continue
		}

		if ev.Change != "title" {
			continue
		}

		if ev.Container.WindowProperties.Class != "Emacs" {
			continue
		}

		matches := titlePattern.FindStringSubmatch(ev.Container.Name)
		if matches == nil {
			continue
		}

		projectName := matches[1]
		projectPath := matches[2]

		if strings.HasPrefix(projectPath, "~") {
			projectPath = filepath.Join(home, projectPath[1:])
		}

		// Emacs mixes these up a bit before a buffer has been selected.
		if !strings.Contains(projectPath, projectName) {
			continue
		}

		// Only react to Emacs windows on the same workspace as kitty.
		kittyWindows, err := listOSWindows(socketPath)
		if err != nil {
			log.Printf("list kitty os windows: %v", err)
			continue
		}

		kittyWindowIDs := make(map[int64]bool, len(kittyWindows))
		for _, ow := range kittyWindows {
			kittyWindowIDs[ow.PlatformWindowID] = true
		}

		root, treeErr := getI3Tree()
		if treeErr != nil {
			log.Printf("get i3 tree: %v", treeErr)
			continue
		}

		emacsWS := findWorkspace(root, "", func(n *i3Node) bool {
			return n.ID == int64(ev.Container.ID)
		})
		kittyWS := findWorkspace(root, "", func(n *i3Node) bool {
			return n.Window != 0 && kittyWindowIDs[n.Window]
		})

		if emacsWS == "" || kittyWS == "" || emacsWS != kittyWS {
			log.Printf("skip %q: emacs on %q, kitty on %q",
				projectName, emacsWS, kittyWS)
			continue
		}

		err = handleProject(socketPath, projectName, projectPath)
		if err != nil {
			log.Printf("handle project %q: %v", projectName, err)
		}

		// Refocus the exact Emacs window that triggered the event.
		_, err = i3.RunCommand(fmt.Sprintf("[con_id=%d] focus", ev.Container.ID))
		if err != nil {
			log.Printf("refocus emacs: %v", err)
		}
	}

	err = recv.Close()
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}
}

func startKitty(socketPath string) error {
	// Clean up any stale socket from a previous run.
	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale socket: %w", err)
	}

	// Ensure the socket parent directory exists.
	err = os.MkdirAll(filepath.Dir(socketPath), 0o700)
	if err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	cmd := exec.Command("kitty",
		"-o", "allow_remote_control=yes",
		"-o", "enabled_layouts=all",
		"--listen-on=unix:"+socketPath,
	)

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("launch kitty: %w", err)
	}

	// When kitty exits, so do we.
	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("kitty exited: %v", err)
			os.Exit(1)
		}

		os.Exit(0)
	}()

	return nil
}

func handleProject(socketPath, projectName, projectPath string) error {
	tabs, err := listTabs(socketPath)
	if err != nil {
		return err
	}

	for _, tab := range tabs {
		if tab.Title == projectName {
			return focusTab(socketPath, tab.ID)
		}
	}

	return createProjectTab(socketPath, projectName, projectPath)
}
