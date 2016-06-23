package node

import (
	"fmt"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func newDemoteCommand(dockerCli *client.DockerCli) *cobra.Command {
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:   "demote NODE [NODE...]",
		Short: "Demote a node from manager in the swarm",
		Args:  cli.RequiresMinArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDemote(dockerCli, flags, args)
		},
	}

	flags = cmd.Flags()
	return cmd
}

func runDemote(dockerCli *client.DockerCli, flags *pflag.FlagSet, args []string) error {
	for _, id := range args {
		if err := runUpdate(dockerCli, id, func(node *swarm.Node) {
			node.Spec.Role = swarm.NodeRoleWorker
		}); err != nil {
			return err
		}
		fmt.Fprintf(dockerCli.Out(), "Manager %s demoted in the swarm.\n", id)
	}

	return nil
}
