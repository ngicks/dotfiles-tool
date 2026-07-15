// Package cli holds terminal presentation helpers kept out of the cmd entrypoint.
package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func Confirm(in io.Reader, out io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprintf(out, "%s [y/N]: ", prompt); err != nil {
		return false, err
	}
	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		return false, sc.Err()
	}
	switch strings.ToLower(strings.TrimSpace(sc.Text())) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
