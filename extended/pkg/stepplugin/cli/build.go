package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newBuildCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "build DIR",
		Short:        "build a StepPlugin",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			pluginDir := args[0]
			plug, err := loadPluginManifest(pluginDir)
			if err != nil {
				return err
			}
			cmPath, err := saveBuiltConfigMap(pluginDir, plug)
			if err != nil {
				return err
			}
			fmt.Printf("%s created\n", cmPath)
			readmePath, err := saveReadme(pluginDir, plug)
			if err != nil {
				return err
			}
			fmt.Printf("%s created\n", readmePath)
			return nil
		},
	}
}
