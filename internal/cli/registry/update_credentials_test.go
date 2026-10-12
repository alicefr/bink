// SPDX-FileCopyrightText: 2026 The bink Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestUpdateCredentialsRequiresUser(t *testing.T) {
	g := NewWithT(t)
	cmd := newUpdateCredentialsCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	g.Expect(cmd.Execute()).To(MatchError(ContainSubstring("required flag")))
}

func TestUpdateCredentialsFlagsDefaultToEmpty(t *testing.T) {
	g := NewWithT(t)
	cmd := newUpdateCredentialsCmd()

	username, err := cmd.Flags().GetString("registry-user")
	g.Expect(err).ToNot(HaveOccurred())
	password, err := cmd.Flags().GetString("registry-password")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(username).To(BeEmpty())
	g.Expect(password).To(BeEmpty())
}
