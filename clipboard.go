package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
)

var (
	urlRegex            = regexp.MustCompile(`(https?://cdn\.discordapp\.com/attachments/[^\s]+)`)
	isListenerRunning   atomic.Bool
	stopListenerChannel = make(chan struct{})
)

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
	if isListenerRunning.CompareAndSwap(false, true) {
		fmt.Println("Starting clipboard listener...")
		stopListenerChannel = make(chan struct{})
		go runClipboardListener()
		fmt.Println("Clipboard listener started. Watching for Discord attachment links...")
	}
}

func stopClipboardListener() {
	if isListenerRunning.CompareAndSwap(true, false) {
		fmt.Println("Stopping clipboard listener...")
		close(stopListenerChannel)
	}
}

func runClipboardListener() {
	var lastContent string
	ticker := time.NewTicker(200 * time.Millisecond) // Poll every 200ms
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			content, err := getClipboardContent()
			if err != nil {
				continue
			}

			if content != "" && content != lastContent {
				lastContent = content
				matches := urlRegex.FindStringSubmatch(content)
				if len(matches) > 1 {
					//fmt.Println("Found Discord link, adding to queue:", matches[1])
					queueMutex.Lock()
					submittedUrlQueue = append(submittedUrlQueue, matches[1])
					queueMutex.Unlock()
				}
			}
		case <-stopListenerChannel:
			fmt.Println("Clipboard listener stopped.")
			return
		}
	}
}
