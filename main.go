// Copyright 2016 The Bench Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Bench is a tool for running package benchmarks in isolation, one by one.
//
// Installation:
//
//	go get [-u] github.com/cznic/bench
//
// Usage:
//
//	bench [-benchmem] [import-path]
//
// Purpose
//
// Sometimes benchmarks influence each other and the results of a particular
// benchmark are way different compared to when that benchmark is run alone.  A
// common, but definitely not the only cause of this is the interference of the
// garbage collector where the previous benchmark stressed the memory usage
// substantially. Manually invoking the GC doesn't seem to always help as it
// looks like it's only a hint to the runtime.  This tool simply runs
// repeatedly the go test command requesting to run all package benchmarks one
// by one. The output is in a format similar to go test and it should be
// benchcmp compatible.
//
// bench does not run any tests.
//
// Example
//
// Benchmarking a branch of github.com/cznic/lldb
//
// 	$ benchcmp -mag -changed log-go-test log-bench
// 	benchmark                                         old ns/op     new ns/op     delta
// 	BenchmarkAllocatorRndGetSimpleFileFiler1e3-4      418091        2217          -99.47%
// 	BenchmarkAllocatorRndFreeSimpleFileFiler1e3-4     31709         8038          -74.65%
// 	BenchmarkAllocatorAllocSimpleFileFiler1e3-4       14292         5675          -60.29%
// 	...
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cznic/gc"
	"github.com/cznic/mathutil"
	"golang.org/x/tools/benchmark/parse"
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-benchmem] [package]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
}

var (
	//TODO oBench = flag.String("bench", ".", "Regexp to select benchmarks.")
	oBenchmem = flag.Bool("benchmem", false, "Print memory allocation statistics for benchmarks.")
)

func defaultTags() []string {
	v := runtime.Version()
	if !strings.HasPrefix(v, "go") {
		log.Panicln("internal error")
	}

	v = v[len("go"):]
	i := 0
outer:
	for i < len(v) {
		switch c := v[i]; {
		case c >= '0' && c <= '9' || c == '.':
			i++
		default:
			break outer
		}
	}
	v = v[:i]
	if v == "" {
		log.Panicln("internal error")
	}

	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Panicln(err)
	}

	var tags []string
	for i := 1; i <= int(10*n+.5)%10; i++ {
		tags = append(tags, fmt.Sprintf("go1.%d", i))
	}
	return tags
}

func main() {
	log.SetFlags(0)
	_, err := exec.LookPath("go")
	if err != nil {
		log.Fatalf("Cannot find the go tool: %s", err)
	}

	flag.Parse()
	var importPath string
	gopaths := filepath.SplitList(os.Getenv("GOPATH"))
	switch {
	case flag.NArg() == 0 || flag.NArg() == 1 && flag.Arg(0) == ".":
		wd, err := os.Getwd()
		if err != nil {
			log.Fatal(err)
		}

		for _, v := range gopaths {
			var err error
			if importPath, err = filepath.Rel(filepath.Join(v, "src"), wd); err == nil {
				break
			}
		}
		if importPath == "" {
			log.Fatal("Cannot determine import path of the current directory.")
		}
	case flag.NArg() == 1:
		importPath = flag.Arg(0)
	default:
		log.Fatal("At most one import path is supported.")
	}

	ctx, err := gc.NewContext(
		runtime.GOOS,
		runtime.GOARCH,
		runtime.GOROOT(),
		gopaths,
		defaultTags(),
	)
	if err != nil {
		log.Fatal(err)
	}

	_, _, testFiles, err := ctx.FilesFromImportPath(importPath)
	if err != nil {
		log.Fatal(err)
	}

	var bench [][]byte
	for _, v := range testFiles {
		b, err := ioutil.ReadFile(v)
		if err != nil {
			log.Fatal(err)
		}

		a := bytes.Split(b, []byte{'\n'})
		for _, v := range a {
			if !bytes.HasPrefix(v, []byte("func Benchmark")) {
				continue
			}

			v = v[len("func "):]
			v = v[:bytes.Index(v, []byte{'('})]
			bench = append(bench, v)
		}
	}
	width := 0
	for _, v := range bench {
		width = mathutil.Max(width, len(v))
	}

	var t time.Duration
	for _, v := range bench {
		args := []string{"test", "-run", "NONE", "-bench", fmt.Sprintf("^%s$", v), importPath}
		if *oBenchmem {
			args = append(args, "-benchmem")
		}
		cmd := exec.Command("go", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatal(err)
		}

		// Inputs
		// ------
		// $
		// Benchmark1-4   	    2000	   1068291 ns/op
		// PASS
		// ok  	github.com/cznic/bench	2.250s
		//
		// Benchmark2-4   	     100	  10067251 ns/op
		// PASS
		// ok  	github.com/cznic/bench	1.021s
		//
		// Collective Output
		// ------
		// $ go test -bench .
		// Benchmark1-4   	    2000	   1067848 ns/op
		// Benchmark2-4   	     100	  10066557 ns/op
		// PASS
		// ok  	github.com/cznic/bench	3.267s
		// $

		a := bytes.Split(out, []byte{'\n'})
		if len(a) < 3 {
			log.Fatalf("Unrecognized format of go test output:\n%s", out)
		}

		p := fmt.Sprintf("ok  \t%s\t", importPath)
		if !bytes.HasPrefix(a[0], v) ||
			!bytes.Equal(a[1], []byte("PASS")) ||
			!bytes.HasPrefix(a[2], []byte(p)) {
			log.Fatalf("Unexpected format of go test output:\n%s", out)
		}

		d, err := time.ParseDuration(string(a[2][len(p):]))
		if err != nil {
			log.Fatalf("Cannot parse benchmark duration\n%s", out)
		}

		t += d
		b, err := parse.ParseLine(string(a[0]))
		if err != nil {
			fmt.Printf("%s\n", a[0])
			continue
		}

		fmt.Printf("%*s%15d", -(width + 4), b.Name, b.N)
		if b.Measured&parse.NsPerOp != 0 {
			s := fmt.Sprintf("%.2f", b.NsPerOp)
			if strings.Index(s, ".") > 2 {
				s = s[:len(s)-3]
			}
			fmt.Printf("%15s ns/op", s)
		}
		if b.Measured&parse.MBPerS != 0 {
			fmt.Printf("%15.2f MB/s", b.MBPerS)
		}
		if b.Measured&parse.AllocedBytesPerOp != 0 {
			fmt.Printf("%15v B/op", b.AllocedBytesPerOp)
		}
		if b.Measured&parse.AllocsPerOp != 0 {
			fmt.Printf("%15v allocs/op", b.AllocsPerOp)
		}
		fmt.Println()
	}
	fmt.Printf("PASS\n")
	fmt.Printf("ok  \t%s\t%v\n", importPath, t)
}
