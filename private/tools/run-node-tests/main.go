package main

import (
	"github.com/latticebuild/rules_js/go/process"
	"github.com/latticebuild/rules_js/private/tools/internal/nodetest"
)

func main() { process.Main(nodetest.Run) }
