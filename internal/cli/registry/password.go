// SPDX-FileCopyrightText: 2026 The bink Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

func readPassword() (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("no password provided and stdin is not a terminal; use --registry-password")
	}

	fmt.Fprint(os.Stderr, "Registry password: ")
	password, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}

	return string(password), nil
}
