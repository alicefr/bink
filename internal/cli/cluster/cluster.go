package cluster

import (
	"github.com/spf13/cobra"
)

func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage Kubernetes cluster",
		Long:  "Create, start, stop, and manage the Kubernetes cluster",
	}

	cmd.AddCommand(newStartCmd())
	cmd.AddCommand(newStopCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}
