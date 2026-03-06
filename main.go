package main

import (
	"fmt"
	"log"
	"os"
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

		err := handleProject(socketPath, projectName, projectPath)
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
