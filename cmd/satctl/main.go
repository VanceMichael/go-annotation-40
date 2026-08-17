// Command satctl 是低轨卫星互联网星座测控与频轨资源调度平台的命令行入口。
package main

import (
	"os"

	"satnet/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args, os.Stdout, os.Stderr))
}
