package main

import "flag"

var logger = newLogger(false)

func main() {
	verbose := flag.Bool("v", false, "log what metago is doing")
	flag.BoolVar(verbose, "verbose", false, "log what metago is doing")
	flag.Parse()

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	logger = newLogger(*verbose)
	logger.Debug("starting metago", "dir", dir)
	if err := run(dir); err != nil {
		fatal(err)
	}
}
