package conversion

import (
	"github.com/alauda/topolvm-operator/pkg/cluster"
	"github.com/alauda/topolvm-operator/pkg/converter"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"os"
)

var ConversionCmd = &cobra.Command{
	Use:   "conversion",
	Short: "crd conversion server",
}

func init() {
	ConversionCmd.RunE = startConversion
}

func startConversion(cmd *cobra.Command, args []string) error {
	var c converter.Config
	c.CertFile = os.Getenv(cluster.ConversionCertFileEnv)
	c.KeyFile = os.Getenv(cluster.ConversionKeyFileEnv)
	if c.CertFile == "" || c.KeyFile == "" {
		return errors.New("must config conversion tls")
	}
	converter.Start(&c)
	return nil
}
