package main

import (
	"fmt"
	"os"

	"github.com/soppylzz/soppy-utils/internal/clioutput"
	"github.com/soppylzz/soppy-utils/internal/configstore"
	"github.com/soppylzz/soppy-utils/internal/gitconfig"
	"github.com/soppylzz/soppy-utils/internal/license"
	"github.com/soppylzz/soppy-utils/internal/welcome"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		clioutput.Error("spu", err.Error())
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing helper command")
	}
	if args[0] == "--version" {
		fmt.Println(version)
		return nil
	}
	home, err := configstore.Home()
	if err != nil {
		return err
	}
	switch args[0] {
	case "license":
		return license.Run(home, args[1:])
	case "welcome":
		welcome.Version = version
		return welcome.Run(home, args[1:])
	case "git":
		return gitconfig.Run(home, args[1:])
	case "config-status":
		for _, s := range []configstore.Store{license.Store(home), welcome.Store(home), gitconfig.Store(home)} {
			v := s.Read()
			if v.Exists && v.Warning != "" {
				clioutput.Warn(s.Util, v.Warning)
			}
		}
		if welcome.LegacyChecksum(home) {
			clioutput.Warn("welcome", "legacy state found; run 'spu welcome migrate'")
		}
		if gitconfig.LegacyPresent(home) {
			clioutput.Warn("git", "legacy state found; run 'spu git migrate'")
		}
		return nil
	default:
		return fmt.Errorf("unknown helper command %q", args[0])
	}
}
