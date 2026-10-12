// SPDX-FileCopyrightText: 2026 The bink Authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"fmt"

	registrypkg "github.com/bootc-dev/bink/internal/registry"
	"github.com/spf13/cobra"
)

func newUpdateCredentialsCmd() *cobra.Command {
	var registryUser string
	var registryPassword string

	cmd := &cobra.Command{
		Use:   "update-credentials",
		Short: "Update the authenticated registry credentials",
		Long:  "Update the credentials for the running authenticated registry without restarting it",
		RunE: func(cmd *cobra.Command, args []string) error {
			if registryPassword == "" {
				p, err := readPassword()
				if err != nil {
					return err
				}
				registryPassword = p
			}

			if err := registrypkg.ValidateAuthCredentials(registryUser, registryPassword); err != nil {
				return fmt.Errorf("invalid credentials: %w", err)
			}

			mgr, err := registrypkg.NewManager()
			if err != nil {
				return fmt.Errorf("creating registry manager: %w", err)
			}

			if err := mgr.UpdateAuthRegistryCredentials(cmd.Context(), registryUser, registryPassword); err != nil {
				return fmt.Errorf("updating auth registry credentials: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&registryUser, "registry-user", "", "Username for the authenticated registry")
	cmd.Flags().StringVar(&registryPassword, "registry-password", "", "Password for the authenticated registry")
	_ = cmd.MarkFlagRequired("registry-user")

	return cmd
}
