package main

import (
	"sort"
)

func findAllCycles(pkgs map[string]*goPackageLock) [][]string {
	var cycles [][]string
	for _, scc := range findStronglyConnectedComponents(pkgs) {
		if len(scc) > 1 {
			sort.Strings(scc)
			cycles = append(cycles, scc)
		}
	}
	return cycles
}

func findStronglyConnectedComponents(pkgs map[string]*goPackageLock) [][]string {
	index := 0
	stack := []string{}
	indices := make(map[string]int)
	lowlinks := make(map[string]int)
	onStack := make(map[string]bool)
	var sccs [][]string

	var strongConnect func(string)
	strongConnect = func(v string) {
		indices[v] = index
		lowlinks[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		pkg, exists := pkgs[v]
		if exists {
			for _, w := range pkg.Require {
				if _, visited := indices[w]; !visited {
					strongConnect(w)
					if lowlinks[w] < lowlinks[v] {
						lowlinks[v] = lowlinks[w]
					}
				} else if onStack[w] {
					if indices[w] < lowlinks[v] {
						lowlinks[v] = indices[w]
					}
				}
			}
		}

		if lowlinks[v] == indices[v] {
			var scc []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, scc)
		}
	}

	for name := range pkgs {
		if _, visited := indices[name]; !visited {
			strongConnect(name)
		}
	}

	return sccs
}
