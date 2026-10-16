// Command spu is the single executable: a client, and the daemon a client starts when
// it cannot reach one.
package main

import (
	"os"

	"github.com/soppylzz/soppy-utils/internal/cli"
)

var version = "dev"

func main() { os.Exit(cli.Main(os.Args, version)) }
