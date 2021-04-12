// Copyright (C) 2009 Alphabet Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*
  Run shell tests in parallel

  Usage:

     cd ~/git/t
     go run ~/vc/rungittest/main.go --outdir results.6cb5e6e7b8e 't00*sh'

  this will run t00*.sh and leave log files in results.6cb5e6e7b8e.
*/

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
	"sort"
	"strings"
	"time"
)

type result struct {
	name    string
	summary string
	err     error
}

func runTest(name, outdir string) *result {
	f, err := os.Create(filepath.Join(outdir, name+".log"))
	if err != nil {
		return &result{
			summary: "create error",
			err:     err,
		}
	}
	defer f.Close()
	cmd := exec.Command("/bin/sh", name)
	outBuf := bytes.Buffer{}
	errBuf := bytes.Buffer{}
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()

	errStr := "success"
	if err != nil {
		errStr = err.Error()
	}
	fmt.Fprintf(f, "*** EXIT: %s ***\n\n", errStr)
	fmt.Fprintf(f, "*** STDOUT: ***\n\n")
	f.Write(outBuf.Bytes())
	fmt.Fprintf(f, "\n\n*** STDERR: ***\n\n")
	f.Write(errBuf.Bytes())

	lines := bytes.Split(outBuf.Bytes(), []byte("\n"))
	summary := ""
	if len(lines) >= 3 {
		lines = lines[len(lines)-3:]
		summary = string(lines[0])
	}

	if err != nil {
		summary = "error: " + summary
	} else {
		summary = "ok: " + summary
	}

	return &result{
		name:    name,
		summary: summary,
		err:     err,
	}
}

func main() {
	jobs := flag.Int("jobs", runtime.NumCPU(), "jobs")
	out := flag.String("outdir", "", "output dir")
	flag.Parse()

	if *out == "" {
		log.Fatalf("must provide --outdir.")
	}
	if len(flag.Args()) == 0 {
		log.Fatalf("usage: provide glob")
	}

	var entries []string
	for _, f := range flag.Args() {
		es, err := filepath.Glob(f)
		if err != nil {
			log.Fatalf("glob: %v", err)
		}
		entries = append(entries, es...)
	}

	if err := os.MkdirAll(*out, 0755); err != nil {
		log.Fatal(err)
	}

	start := time.Now()
	N := len(entries)
	throttle := make(chan int, *jobs)
	results := make(chan *result, N)
	for _, e := range entries {
		go func(nm string) {
			throttle <- 1
			defer func() { <-throttle }()

			results <- runTest(nm, *out)
		}(e)
	}

	var failed []string
	for i := range entries {
		r := <-results
		fmt.Printf("\r%d/%d: %-20s - %-60s ", i+1, N, r.name, r.summary)
		if r.err != nil {
			failed = append(failed, r.name)
			fmt.Println()
		}
	}
	fmt.Println()

	sort.Strings(failed)
	elapsed := time.Now().Sub(start)
	if err := ioutil.WriteFile(filepath.Join(*out, "summary.txt"),
		[]byte(fmt.Sprintf("# run %s\n# on %s, elapsed %s:\n%s",
			os.Args, time.Now().Format(time.RFC3339), elapsed,
			strings.Join(failed, "\n"))), 0644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%d failures, elapsed %s\n", len(failed), elapsed)
}
