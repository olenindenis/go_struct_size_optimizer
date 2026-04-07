package main

import (
	"flag"
	"fmt"
	"go/token"
	"log"

	"github.com/olenindenis/go-struct-size-optimizer/internal/analyzer"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

func main() {
	var searchPath string
	flag.StringVar(&searchPath, "path", "", "directory to analyze (default: current directory)")
	flag.BoolVar(&analyzer.WriteMode, "w", false, "write result to file")
	flag.Parse()

	patterns := flag.Args()
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}

	cfg := &packages.Config{
		Mode: packages.LoadSyntax,
		Dir:  searchPath,
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		log.Fatal(err)
	}

	var lastFset *token.FileSet
	for _, pkg := range pkgs {
		pass := &analysis.Pass{
			Fset:       pkg.Fset,
			Files:      pkg.Syntax,
			TypesInfo:  pkg.TypesInfo,
			TypesSizes: pkg.TypesSizes,
			Report: func(d analysis.Diagnostic) {
				fmt.Printf("%s: %s\n", pkg.Fset.Position(d.Pos), d.Message)
			},
		}
		if pass.TypesSizes == nil {
			continue
		}

		lastFset = pkg.Fset
		if _, err = analyzer.Analyzer.Run(pass); err != nil {
			log.Fatal(err)
		}
	}

	if analyzer.WriteMode {
		if lastFset == nil {
			log.Fatal("no packages loaded")
		}
		if err := analyzer.ApplyEdits(lastFset); err != nil {
			log.Fatal(err)
		}
	} else {
		fmt.Println("analysis complete")
	}
}
