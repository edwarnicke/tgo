// Copyright (c) 2020 Cisco and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"log"
	"os"
	"os/exec"

	"github.com/edwarnicke/tgo"
)

func main() {
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	cache := tgo.New(pwd)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bug", "build", "doc", "env", "fix", "fmt", "generate", "get", "install", "list", "mod", "run", "test", "tool", "version", "vet", "help":
			exitOnErr(cache.RunArgs("go", os.Args[1:]...))
		case "clean":
			exitOnErr(cache.RunArgs("go", os.Args[1:]...))
			if len(os.Args) == 2 {
				exitOnErr(cache.Clean())
			}
		default:
			exitOnErr(cache.Run("go build ./..."))
			if _, err := exec.LookPath(os.Args[1]); err != nil {
				os.Exit(1)
			}
			exitOnErr(cache.RunArgs(os.Args[1], os.Args[2:]...))
		}
	} else {
		exitOnErr(cache.Run("go build ./..."))
	}
}

func exitOnErr(err error) {
	if exit, ok := err.(*exec.ExitError); ok {
		log.Println(err)
		os.Exit(exit.ExitCode())
	}
}
