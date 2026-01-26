package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	expect "github.com/Netflix/go-expect"
	"github.com/creack/pty"
)

// Observer simulates the LLM decision maker
type Observer struct{}

func (o *Observer) Evaluate(context string) bool {
	// Clean ANSI codes for analysis
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	cleanText := ansiRegex.ReplaceAllString(context, "")
	cleanText = strings.TrimSpace(cleanText)

	fmt.Printf("\n[Go-Observer] Analyzing request...\n")
	// fmt.Printf("[Go-Observer] Context: %s...\n", cleanText[:min(200, len(cleanText))])

	if strings.Contains(cleanText, "rm -rf") {
		fmt.Println("[Go-Observer] 🛑 BLOCKED: High risk command detected.")
		return false
	}

	if strings.Contains(strings.ToLower(cleanText), "create") ||
		strings.Contains(strings.ToLower(cleanText), "write") ||
		strings.Contains(strings.ToLower(cleanText), "cost") {
		fmt.Println("[Go-Observer] ✅ APPROVED: Safe operation.")
		return true
	}

	fmt.Println("[Go-Observer] ⚠️ UNCERTAIN: Requesting manual review (Simulating 'Allow').")
	return true
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func RunSafeClaude(initialPrompt string) {
	fmt.Printf("[Go-Wrapper] Starting 'claude'...\n")

	// Create a new console using go-expect which manages the PTY
	c, err := expect.NewConsole(expect.WithStdout(os.Stdout), expect.WithDefaultTimeout(60*time.Second))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	// Direct pty command to handle process lifecycle better if needed,
	// but go-expect can also wrap exec.Cmd.
	// Here we spawn the process manually and link it to the console's Tty.
	cmd := exec.Command("claude")
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	// We start the pty using creack/pty to get the pty file
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Fatal(err)
	}
	defer ptmx.Close()

	// Link expect console to the pty master
	// Note: netflix/go-expect's usually assumes IT creates the pty or you pass the file.
	// Actually, careful here: expect.NewConsole creates a pair.
	// We want to run the command attached to that pair.
	// Let's use the simpler pattern provided by existing examples or just re-map.
	// RE-BINDING STRATEGY:
	// The best way with standard 'go-expect' is to copy streams, but we want a PTY.
	// Let's restart with the cleaner pattern: use `c.Tty()` as the slave.

	// Wait.. expect.NewConsole() creates a Master (c.File) and returns it.
	// No, it calls pty.Open() internally.

	// Let's do this:
	// We already created 'c'. cmd should use c.Tty() as its stdio.
	// But c.Tty() is the slave. Correct.

	// Re-configuring cmd to use the console's slave
	cmd = exec.Command("claude")
	slave := c.Tty()
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave

	if err := cmd.Start(); err != nil {
		log.Fatal(err)
	}

	// 1. Handle Startup sequence (Trust prompt / Banner)
	// We loop looking for the ready prompt ">"

	// go-expect's `Expect` function blocks until match.
	// We need multiple matchers.

	observer := &Observer{}

	// Startup Loop
	fmt.Println("[Go-Wrapper] Waiting for startup...")

	// Depending on state, we might hit:
	// - "Do you trust..."
	// - ">" (ready)
	// - "Login..."

	// Note: Batch expectation in go-expect is handled nicely.
	for {
		// Expect Batch
		// We look for either the Prompt or the Trust request
		val, err := c.Expect(expect.RegexpPattern(`(> $|Do you trust|Visit .* to log in|Welcome to Claude Code)`))
		if err != nil {
			log.Printf("Error expecting: %v\n", err)
			return
		}

		if strings.Contains(val, "Do you trust") {
			fmt.Println("\n[Go-Wrapper] Trust prompt detected. Accepting...")
			c.SendLine("") // Enter
			continue
		} else if strings.Contains(val, "Visit") {
			fmt.Println("\n[Go-Wrapper] 🚨 Auth Required!")
			return
		} else if strings.Contains(val, "Welcome") {
			// just wait more
			continue
		} else {
			// Matched ">"
			fmt.Println("\n[Go-Wrapper] Ready. Sending prompt...")
			break
		}
	}

	// Send the user prompt
	c.SendLine(initialPrompt)

	// Interaction Loop
	for {
		// We expect either:
		// 1. Permission request [y/N]
		// 2. The prompt ">" indicating done
		// 3. EOF/Termination

		// Regex for y/n variants
		// Go regexp: `(?i)(\[y/n\]|\(y/n\))`
		permissionPattern := `(?:\[y/N\]|\(y/n\))`
		promptPattern := `> $`

		val, err := c.Expect(expect.RegexpPattern(fmt.Sprintf("(%s|%s)", permissionPattern, promptPattern)))

		if err != nil {
			// EOF or error
			fmt.Println("\n[Go-Wrapper] Command exited or error.")
			break
		}

		// Check what we matched (idx is NOT returned by single Expect(RegexpPattern),
		// Expect returns output, match string, error.
		// NOTE: My understanding of netflix/go-expect API might be slightly off on 'idx'.
		// The `Case` API is better for multiple branches.

		// Let's check `val` (the output text matched + previous buffer).

		// The library returns the full buffer up to match + the match itself.
		// We verify the end of the string.

		// To match multiple things properly, we should use c.Expect(expect.OneOf(...)) likely?
		// But let's keep it simple: Regex OR.

		matchedText := val

		isPermission := regexp.MustCompile(permissionPattern).MatchString(matchedText)
		isPrompt := regexp.MustCompile(promptPattern).MatchString(matchedText)

		// Heuristic: If prompt is at the very end and permission pattern isn't there (or came before?)
		// Permission prompt usually ends the line.

		if isPermission {
			fmt.Printf("\n[Go-Wrapper] 🛡️  INTERCEPTION: Permission requested.\n")
			allowed := observer.Evaluate(matchedText)
			if allowed {
				c.SendLine("y")
			} else {
				c.SendLine("n")
			}
		} else if isPrompt {
			// If we matched the prompt, tasks is likely done.
			// However, sometimes permission request might be embedded? Unlikely.
			// Assuming done.
			fmt.Println("\n[Go-Wrapper] Task complete.")
			break
		}
	}

	// Cleanup
	cmd.Process.Kill()
}

func main() {
	cmd := "echo 'Hello from Go Wrapper'"
	if len(os.Args) > 1 {
		cmd = strings.Join(os.Args[1:], " ")
	}
	RunSafeClaude(cmd)
}
