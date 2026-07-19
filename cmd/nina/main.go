package main

import (
	"fmt"
	"os"
)

const version = "0.0.1-skeleton"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "nina:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Println("nina", version)
		return nil
	case "start":
		if len(args) < 2 {
			return fmt.Errorf("start requires a learning goal, e.g. nina start \"learn Python basics\"")
		}
		return fmt.Errorf("start is not implemented yet")
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Println(`nina — AI pair programming companion

Usage:
  nina start "<learning goal>"   begin a guided session in the current directory
  nina version                   print version`)
}
