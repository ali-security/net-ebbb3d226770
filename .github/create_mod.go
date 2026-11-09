// Command create_mod packs a Go module directory into the module zip format
// served by proxy.golang.org, using golang.org/x/mod/zip.CreateFromDir.
//
// Usage: create_mod <module-path> <version> <module-dir> <out.zip>
package main

import (
	"fmt"
	"os"

	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: create_mod <module-path> <version> <module-dir> <out.zip>")
		os.Exit(2)
	}
	f, err := os.Create(os.Args[4])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	mv := module.Version{Path: os.Args[1], Version: os.Args[2]}
	if err := zip.CreateFromDir(f, mv, os.Args[3]); err != nil {
		f.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := f.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
