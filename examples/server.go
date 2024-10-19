package main

import (
	"github.com/burkel24/go-mochi"
	"go.uber.org/fx"
)

func main() {
	appOpts := mochi.BuildAppOpts()
	serverOpts := mochi.BuildServerOpts()

	allOpts := append(appOpts, serverOpts...)

	fx.New(allOpts...).Run()
}
