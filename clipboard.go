package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var urlRegex = regexp.MustCompile(`(https?://cdn\.discordapp\.com/attachments/[^\s]+)`)

func getClipboardContent() (string, error) {
	cmd := exec.Command("powershell", "-Command", "Get-Clipboard")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func startClipboardListener() {
	var lastContent string
	fmt.Println("Clipboard listener started. Watching for Discord attachment links...")

	go func() {
		for {
			content, err := getClipboardContent()
			if err != nil {
				// Ignore errors, just try again
				time.Sleep(2 * time.Second)
				continue
			}

			if content != "" && content != lastContent {
				lastContent = content
				matches := urlRegex.FindStringSubmatch(content)
				if len(matches) > 1 {
					fmt.Println("Found Discord link, adding to queue:", matches[1])
					queueMutex.Lock()
					submittedUrlQueue = append(submittedUrlQueue, matches[1])
					queueMutex.Unlock()
				}
			}

			time.Sleep(2 * time.Second)
		}
	}()
}
