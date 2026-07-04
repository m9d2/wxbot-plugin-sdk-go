package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: wxbot-plugin init|build|package|pack|validate|verify|publish")
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "build":
		return runBuild(args[1:])
	case "package":
		return runPackage(args[1:])
	case "pack":
		return runPack(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "verify":
		return runVerify(args[1:])
	case "publish":
		return runPublish(args[1:])
	default:
		return fmt.Errorf("未知命令: %s", args[0])
	}
}
