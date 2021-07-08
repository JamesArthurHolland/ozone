package cli

import (
	"fmt"
	process_manager_client "github.com/JamesArthurHolland/ozone/ozone-daemon-lib/process-manager-client"
	"github.com/spf13/cobra"
	"log"
)

func init() {
	rootCmd.AddCommand(contextCmd)
}

var contextCmd = &cobra.Command{
	Use:   "c",
	Long:  `Show context or change context`,
	Run: func(cmd *cobra.Command, args []string) {

		if len(args) == 0 {
			currentContext, err := process_manager_client.FetchContext(ozoneWorkingDir)
			if err != nil {
				log.Fatalln(err)
			}
			if currentContext == "" {
				currentContext = config.ContextInfo.Default
			}

			for _, context := range config.ContextInfo.List {
				if context == currentContext {
					fmt.Printf("%s \t\t*\n", currentContext)
				} else {
					fmt.Println(context)
				}
			}
		} else {
			givenContext := args[0]
			// TODO check context is in
			if config.HasContext(givenContext) {
				process_manager_client.SetContext(ozoneWorkingDir, givenContext)
				fmt.Printf("Switch to context: '%s'", givenContext)
			} else {
				fmt.Printf("Context '%s' doesn't exist in Ozonefile.", givenContext)
			}
		}
	},
}