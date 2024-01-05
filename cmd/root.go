package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(CreateWalletCmd)
	RootCmd.AddCommand(MintCmd)
}

var (
	RootCmd = &cobra.Command{
		Use: "noss",
		Run: func(cmd *cobra.Command, args []string) {
			// Do Stuff Here
		},
	}
)



