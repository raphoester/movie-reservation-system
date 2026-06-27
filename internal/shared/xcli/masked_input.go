package xcli

import (
	"bufio"
	"fmt"
	"os"

	"golang.org/x/term"
)

// ReadMasked prints prompt, reads a line from stdin, and echoes '*' for each
// character typed. Returns ("", ErrInterrupted) on Ctrl-C.
// Falls back to plain bufio.Scanner when stdin is not a terminal.
func ReadMasked(prompt string) (string, error) {
	fmt.Print(prompt)

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return scanner.Text(), nil
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return "", nil
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			return scanner.Text(), nil
		}
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return "", nil
	}
	defer term.Restore(fd, oldState) //nolint:errcheck

	var input []byte
	buf := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(buf); err != nil {
			fmt.Println()
			return string(input), nil
		}
		switch buf[0] {
		case '\r', '\n':
			fmt.Println()
			return string(input), nil
		case 127, '\b':
			if len(input) > 0 {
				input = input[:len(input)-1]
				fmt.Print("\b \b")
			}
		case 3: // Ctrl-C
			fmt.Println()
			return "", ErrInterrupted
		default:
			input = append(input, buf[0])
			fmt.Print("*")
		}
	}
}
