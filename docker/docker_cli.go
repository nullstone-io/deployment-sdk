package docker

import (
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/nullstone-io/deployment-sdk/logging"
	"github.com/spf13/pflag"
)

func DiscoverDockerCli(osWriters logging.OsWriters) (*command.DockerCli, error) {
	cliOpts := make([]command.DockerCliOption, 0)
	if osWriters != nil {
		if outWriter := osWriters.Stdout(); outWriter != nil {
			cliOpts = append(cliOpts, command.WithOutputStream(outWriter))
		}
		if errWriter := osWriters.Stderr(); errWriter != nil {
			cliOpts = append(cliOpts, command.WithOutputStream(errWriter))
		}
	}
	dockerCli, err := command.NewDockerCli(cliOpts...)
	if err != nil {
		return nil, err
	}
	opts := &flags.ClientOptions{
		Common: &flags.CommonOptions{
			Debug: true,
		},
	}
	opts.Common.InstallFlags(pflag.NewFlagSet("", pflag.ContinueOnError))
	opts.Common.SetDefaultOptions(&pflag.FlagSet{})
	if err := dockerCli.Initialize(opts); err != nil {
		return nil, err
	}
	return dockerCli, nil
}
