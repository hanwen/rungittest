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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

	fmt.Fprintf(f, "\n\n*** STDOUT: ***\n\n")
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
	glob := flag.Args()[0]
	entries, err := filepath.Glob(glob)
	if err != nil {
		log.Fatalf("glob: %v", err)
	}

	if err := os.MkdirAll(*out, 0755); err != nil {
		log.Fatal(err)
	}

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

	for i := range entries {
		r := <-results
		fmt.Printf("\r%d/%d: %-20s - %-60s ", i+1, N, r.name, r.summary)
		if r.err != nil {
			fmt.Println()
		}
	}
	fmt.Println()
}
