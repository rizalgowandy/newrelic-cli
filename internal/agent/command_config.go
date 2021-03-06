package agent

import (
	"github.com/spf13/cobra"

	"github.com/newrelic/newrelic-cli/internal/output"
	"github.com/newrelic/newrelic-cli/internal/utils"
)

var (
	encodeKey    string
	textToEncode string
)

var cmdConfig = &cobra.Command{
	Use:   "config",
	Short: "Configuration utilities/helpers for New Relic agents",
}

var cmdConfigObfuscate = &cobra.Command{
	Use:   "obfuscate",
	Short: "Obfuscate a configuration value using a key",
	Long: `Obfuscate a configuration value using a key.  The obfuscated value
should be placed in the Agent configuration or in an environment variable." 
`,
	Example: "newrelic agent config obfuscate --value <config_value> --key <obfuscation_key>",
	Run: func(cmd *cobra.Command, args []string) {

		result := ObfuscationResult{
			ObfuscatedValue: obfuscateStringWithKey(textToEncode, encodeKey),
		}

		utils.LogIfFatal(output.Print(result))
	},
}

func init() {

	Command.AddCommand(cmdConfig)

	cmdConfig.AddCommand(cmdConfigObfuscate)

	cmdConfigObfuscate.Flags().StringVarP(&encodeKey, "key", "k", "", "the key to use when obfuscating the clear-text value")
	cmdConfigObfuscate.Flags().StringVarP(&textToEncode, "value", "v", "", "the value, in clear text, to be obfuscated")

	utils.LogIfError(cmdConfigObfuscate.MarkFlagRequired("key"))
	utils.LogIfError(cmdConfigObfuscate.MarkFlagRequired("value"))
}
