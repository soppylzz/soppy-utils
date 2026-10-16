// Command spu-helper is the transitional name of the spu binary. It is being renamed to
// spu, and this one stays until the launcher, the installer, and the release archives
// have all moved.
package main

import (
	"os"

	"github.com/soppylzz/soppy-utils/internal/cli"
)

var version = "dev"

func main() { os.Exit(cli.Main(os.Args, version)) }
