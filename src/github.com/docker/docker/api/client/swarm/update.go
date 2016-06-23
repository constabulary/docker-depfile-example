package swarm

import (
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/client"
	"github.com/docker/docker/cli"
	"github.com/docker/engine-api/types/swarm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type updateOptions struct {
	autoAccept          AutoAcceptOption
	secret              string
	taskHistoryLimit    int64
	dispatcherHeartbeat time.Duration
	nodeCertExpiry      time.Duration
}

func newUpdateCommand(dockerCli *client.DockerCli) *cobra.Command {
	opts := updateOptions{autoAccept: NewAutoAcceptOption()}
	var flags *pflag.FlagSet

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the Swarm",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(dockerCli, flags, opts)
		},
	}

	flags = cmd.Flags()
	flags.Var(&opts.autoAccept, "auto-accept", "Auto acceptance policy (worker, manager or none)")
	flags.StringVar(&opts.secret, "secret", "", "Set secret value needed to accept nodes into cluster")
	flags.Int64Var(&opts.taskHistoryLimit, "task-history-limit", 10, "Task history retention limit")
	flags.DurationVar(&opts.dispatcherHeartbeat, "dispatcher-heartbeat", time.Duration(5*time.Second), "Dispatcher heartbeat period")
	flags.DurationVar(&opts.nodeCertExpiry, "cert-expiry", time.Duration(90*24*time.Hour), "Validity period for node certificates")
	return cmd
}

func runUpdate(dockerCli *client.DockerCli, flags *pflag.FlagSet, opts updateOptions) error {
	client := dockerCli.Client()
	ctx := context.Background()

	swarm, err := client.SwarmInspect(ctx)
	if err != nil {
		return err
	}

	err = mergeSwarm(&swarm, flags)
	if err != nil {
		return err
	}

	err = client.SwarmUpdate(ctx, swarm.Version, swarm.Spec)
	if err != nil {
		return err
	}

	fmt.Println("Swarm updated.")
	return nil
}

func mergeSwarm(swarm *swarm.Swarm, flags *pflag.FlagSet) error {
	spec := &swarm.Spec

	if flags.Changed("auto-accept") {
		value := flags.Lookup("auto-accept").Value.(*AutoAcceptOption)
		spec.AcceptancePolicy.Policies = value.Policies(nil)
	}

	var psecret *string
	if flags.Changed("secret") {
		secret, _ := flags.GetString("secret")
		psecret = &secret
	}

	for i := range spec.AcceptancePolicy.Policies {
		spec.AcceptancePolicy.Policies[i].Secret = psecret
	}

	if flags.Changed("task-history-limit") {
		spec.Orchestration.TaskHistoryRetentionLimit, _ = flags.GetInt64("task-history-limit")
	}

	if flags.Changed("dispatcher-heartbeat") {
		if v, err := flags.GetDuration("dispatcher-heartbeat"); err == nil {
			spec.Dispatcher.HeartbeatPeriod = uint64(v.Nanoseconds())
		}
	}

	if flags.Changed("cert-expiry") {
		if v, err := flags.GetDuration("cert-expiry"); err == nil {
			spec.CAConfig.NodeCertExpiry = v
		}
	}

	return nil
}
