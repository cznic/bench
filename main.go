// Copyright 2016 The Bench Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [flags] [packages]\n\n", os.Args[0])
		flag.PrintDefaults()
	}
}

var (
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
	if flag.NArg() != 0 {
		log.Fatal("No non-flag arguments are supported")
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	gopaths := filepath.SplitList(os.Getenv("GOPATH"))
	var importPath string
	for _, v := range gopaths {
		v = filepath.Join(v, "src")
		var err error
		if importPath, err = filepath.Rel(v, wd); err == nil {
			break
		}
	}
	if importPath == "" {
		log.Fatal("Cannot determine import path of the current directory.")
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

	var t time.Duration
	for _, v := range bench {
		args := []string{"test", "-run", "NONE", "-bench", fmt.Sprintf("^%s$", v)}
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
		fmt.Printf("%s\n", a[0])
	}
	fmt.Printf("PASS\n")
	fmt.Printf("ok  \t%s\t%v\n", importPath, t)
}
