// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The go-cacher binary is a cacher helper program that cmd/go can use.
package main

import (
	"log"
	"os"
	"strconv"
	"strings"
	"github.com/adisbladis/gobuild.nix/go/gobuild-nix-gocacheprog/cacheproc"
	"github.com/adisbladis/gobuild.nix/go/gobuild-nix-gocacheprog/cachers"
)

func main() {
	dc := &cachers.DiskCache{}

	// Remove timestamps & extra info from logging
	log.SetFlags(0)
	log.SetPrefix("gobuild.nix: ")

	// Directories containing existing build caches
	if s := os.Getenv("NIX_GOBUILD_CACHE"); s != "" {
		dirs := strings.Split(s, ":")

		dc.InputDirs = dirs

		log.Printf("Using cache inputs:")
		for _, dir := range dirs {
			log.Printf("%v ...", dir)
		}
	}

	// Output build cache
	if dir := os.Getenv("NIX_GOBUILD_CACHE_OUT"); dir != "" {
		dc.OutDir = dir

		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatal(err)
		}

		dc.InputDirs = append(dc.InputDirs, dir)

		log.Printf("Using cache output: %v ...", dir)
	}

	// Timestamp
	if s := os.Getenv("SOURCE_DATE_EPOCH"); s != "" {
		i, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		dc.TimeNanos = i * 1_000_000_000
		log.Printf("Using cache timestamp %v ...", dc.TimeNanos)
	}

	// Output build cache
	if s := os.Getenv("NIX_GOBUILD_CACHE_VERBOSE"); s != "" {
		i, err := strconv.Atoi(s)
		if err != nil {
			log.Fatal(err)
		}
		dc.Verbose = i > 0
	}

	var p *cacheproc.Process
	p = &cacheproc.Process{
		Close: func() error {
			log.Printf("closing; %d gets (%d hits, %d misses, %d errors); %d puts (%d errors)",
				p.Gets.Load(), p.GetHits.Load(), p.GetMisses.Load(), p.GetErrors.Load(), p.Puts.Load(), p.PutErrors.Load())

			// Wait for in-flight writes to finish
			dc.Wait()

			return nil
		},
		Get: dc.Get,
		Put: dc.Put,
	}

	if err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
