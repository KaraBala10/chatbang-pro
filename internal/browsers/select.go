package browsers

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const manualLabel = "Log in manually in a new window"

// PromptSelect prints discovered browsers and reads a numbered choice.
// A nil Source means the user chose manual login.
func PromptSelect(w io.Writer, r io.Reader, sources []Source) (*Source, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	fmt.Fprintln(w, "Found browsers:")
	for i, src := range sources {
		fmt.Fprintf(w, "  %d) %s\n", i+1, src.Label())
	}
	manualN := len(sources) + 1
	fmt.Fprintf(w, "  %d) %s\n", manualN, manualLabel)
	fmt.Fprintln(w)

	scanner := bufio.NewScanner(r)
	for {
		fmt.Fprintf(w, "Choose [%d-%d]: ", 1, manualN)
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return nil, err
			}
			return nil, io.EOF
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > manualN {
			fmt.Fprintf(w, "Please enter a number from 1 to %d.\n", manualN)
			continue
		}
		if n == manualN {
			return nil, nil
		}
		src := sources[n-1]
		return &src, nil
	}
}
