package cmd

import (
	"bytes"
	"fmt"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
	"github.com/spf13/cobra"
	"os"
)

func init() {
	CreateWalletCmd.Flags().IntVar(&flagWalletCount, "count", 1, "wallet count")
}

var (
	flagWalletCount int

	CreateWalletCmd = &cobra.Command{
		Use:   "createwallet",
		Short: "Create a new wallet",
		Long:  `Create a new wallet`,
		Run: func(cmd *cobra.Command, args []string) {

			data := bytes.NewBuffer(nil)
			data.WriteString("nPublicKey,nSecretKey,PublicKey,SecretKey\n")
			for i := 0; i < flagWalletCount; i++ {
				sk := nostr.GeneratePrivateKey()
				pk, _ := nostr.GetPublicKey(sk)
				nsec, _ := nip19.EncodePrivateKey(sk)
				npub, _ := nip19.EncodePublicKey(pk)

				data.WriteString(fmt.Sprintf("%s,%s,%s,%s\n", npub, nsec, pk, sk))
			}

			_ = os.WriteFile("wallet.csv", data.Bytes(), 0644)
		},
	}
)


