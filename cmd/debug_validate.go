package cmd

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/andrewhowdencom/ruf/internal/model"
	"github.com/andrewhowdencom/ruf/internal/sourcer"
	"github.com/andrewhowdencom/ruf/internal/validator"
	"github.com/spf13/cobra"
)

// debugValidateCmd represents the debug validate command
var debugValidateCmd = &cobra.Command{
	Use:   "validate [uri]",
	Short: "Validate a calls file.",
	Long:  `Validate a calls file.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uri := args[0]

		fetcher := sourcer.NewCompositeFetcher()
		fetcher.AddFetcher("http", sourcer.NewHTTPFetcher())
		fetcher.AddFetcher("https", sourcer.NewHTTPFetcher())
		fetcher.AddFetcher("file", sourcer.NewFileFetcher())
		// Not including git fetcher for now, as it requires more configuration

		// Get the path to the current source file, and then find the schema file relative to that.
		_, b, _, _ := runtime.Caller(0)
		basepath := filepath.Dir(b)
		schemaPath := filepath.Join(basepath, "..", "schema", "calls.json")

		parser, err := sourcer.NewYAMLParser(schemaPath)
		if err != nil {
			return fmt.Errorf("failed to create parser: %w", err)
		}
		s := sourcer.NewSourcer(fetcher, parser)

		source, _, err := s.Source(uri)
		if err != nil {
			return err
		}

		if source == nil {
			return nil
		}

		// Create a slice of pointers for validation
		callsToValidate := make([]*model.Call, len(source.Calls))
		for i := range source.Calls {
			callsToValidate[i] = &source.Calls[i]
		}

		errs := validator.Validate(callsToValidate)
		if len(errs) > 0 {
			var errStrings []string
			for _, err := range errs {
				errStrings = append(errStrings, err.Error())
			}
			return fmt.Errorf("validation failed:\n%s", strings.Join(errStrings, "\n"))
		}

		fmt.Fprintln(cmd.OutOrStdout(), "OK")
		return nil
	},
}

func init() {
	debugCmd.AddCommand(debugValidateCmd)
}
