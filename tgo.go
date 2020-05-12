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

// Package tgo provides a mechanism for building an indirectory go cache (source and binaries) transparently
// in a manner that eases use of go with docker.
package tgo

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/edwarnicke/exechelper"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

const (
	pwdEnv    = "PWD"
	goPathEnv = "GOPATH"
)

// Tgo provides a mechanism for building an indirectory go cache (source and binaries) transparently
// in a manner that eases use of go with docker.
type Tgo struct {
	tGoParent string
	tGoDir    string
	tGoRoot   string
	config    map[string]string
	goEnv     map[string]string
	once      sync.Once
	err       error
}

// New - a new Tgo cache
func New(pwd string) *Tgo {
	t := &Tgo{
		tGoParent: pwd,
		tGoDir:    filepath.Join(pwd, ".tgo"),
		tGoRoot:   filepath.Join(pwd, ".tgo", "root"),
		config:    make(map[string]string),
	}
	return t
}

func (t *Tgo) init() error {
	t.once.Do(func() {
		// Read the config
		config, err := godotenv.Read(filepath.Join(t.tGoDir, "env"))
		if err == nil {
			t.config = config
		}
		if err != nil && !os.IsNotExist(err) {
			t.err = err
			return
		}

		// Grab the go envs from go
		output, err := exechelper.Output("go env", exechelper.WithEnvirons(os.Environ()...))
		if err != nil {
			t.err = err
			return
		}
		env, err := godotenv.Unmarshal(string(output))
		if err != nil {
			t.err = err
			return
		}
		t.goEnv = env

		// Initialize .tgo if it didn't exist before
		if _, ok := t.config[pwdEnv]; !ok {
			t.config[pwdEnv] = t.tGoParent
			t.config[goPathEnv] = t.goEnv[goPathEnv]
			if err := t.mkdirs(); err != nil {
				t.err = err
				return
			}
			if err := t.mksymlink(); err != nil {
				t.err = err
				return
			}
			if err := godotenv.Write(t.config, filepath.Join(t.tGoDir, "env")); err != nil {
				t.err = err
				return
			}
		}
		// Load source
		if t.config[pwdEnv] == t.tGoParent {
			if err := t.copysource(); err != nil {
				t.err = err
				return
			}
		}
	})
	return t.err
}

// Run - run in the TGo cache in exechelper style
func (t *Tgo) Run(cmdString string, options ...*exechelper.Option) error {
	if err := t.init(); err != nil {
		return err
	}
	options = append([]*exechelper.Option{
		exechelper.WithEnvirons(os.Environ()...),
		exechelper.WithStdout(os.Stdout),
		exechelper.WithStderr(os.Stderr),
		exechelper.WithStdin(os.Stdin),
		exechelper.WithEnvKV(goPathEnv, t.tGoPath(t.config[goPathEnv])),
		exechelper.WithEnvKV(pwdEnv, t.tGoPath(t.config[pwdEnv])),
	}, options...)
	if err := exechelper.Run(cmdString, options...); err != nil {
		return errors.Wrapf(err, "Error running %s", cmdString)
	}
	return nil
}

// Clean - removes the .tgo directory
func (t *Tgo) Clean() error {
	return t.clean(t.tGoDir)
}

func (t *Tgo) clean(dir string) error {
	if !strings.HasPrefix(dir, t.tGoDir) {
		return errors.Errorf("Cannot clean %q as it is not under %q", dir, t.tGoRoot)
	}
	if err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&0200 == 0 {
			if err := os.Chmod(path, info.Mode()|0200); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return nil
}

func (t *Tgo) tGoPath(path string) string {
	return filepath.Join(t.tGoRoot, path)
}

func (t *Tgo) mkdirs() error {
	for _, dir := range []string{filepath.Dir(t.config[pwdEnv]), t.config[goPathEnv]} {
		if err := os.MkdirAll(t.tGoPath(dir), 0750); err != nil {
			return err
		}
		info, err := os.Stat(dir)
		if err != nil {
			return err
		}
		if err := os.Chmod(t.tGoPath(dir), info.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tgo) mksymlink() error {
	// Symlink to the tgoParent
	relPwd, err := filepath.Rel(t.tGoPath(filepath.Dir(t.tGoParent)), t.tGoParent)
	if err != nil {
		return err
	}
	if err := os.Symlink(relPwd, t.tGoPath(t.tGoParent)); err != nil {
		return err
	}
	return nil
}

func (t *Tgo) copysource() error {
	// Cache the source and binaries
	options := []*exechelper.Option{
		exechelper.WithEnvirons(os.Environ()...),
		exechelper.WithStderr(os.Stderr),
	}
	// Get all the package depended no in tgoParent
	output, err := exechelper.Output(`go list -f '{{if .Module}}{{printf "%s\n%s" .Dir .Module.GoMod }}{{else}}{{.Dir}}{{end}}' all ./...`, options...)
	if err != nil {
		return err
	}

	// Extract the directories from output
	dirs := strings.Split(strings.TrimSpace(string(output)), "\n")
	// Sort the dirs, because it allows us to skip recopying subdirs of dirs we already copied
	sort.Strings(dirs)
	var dirPrefix string
	for _, dir := range dirs {
		// Leave GOROOT and GOPATH out of this... GOPATH can be reconstructed from within the Tgo directory
		if strings.HasPrefix(dir, t.goEnv["GOROOT"]) || strings.HasPrefix(dir, t.goEnv[goPathEnv]) || strings.HasPrefix(dir, t.tGoParent) {
			continue
		}
		// Copy all other source code in
		if dirPrefix == "" || !strings.HasPrefix(dir, dirPrefix) {
			if err := t.clean(t.tGoPath(dir)); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := filepath.Walk(dir, t.sourceCopyWalkFunc); err != nil {
				return err
			}
			dirPrefix = dir
		}
	}
	return nil
}

func (t *Tgo) sourceCopyWalkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if strings.HasSuffix(path, ".tgo") || strings.HasSuffix(path, ".git") ||
		strings.Contains(path, ".tgo"+string(os.PathSeparator)) || strings.Contains(path, ".git"+string(os.PathSeparator)) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(t.tGoPath(path)), 0700); err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(t.tGoPath(path), info.Mode()|0200); err != nil {
			return err
		}
	}
	if info.Mode().IsRegular() {
		src, fileErr := os.Open(path) // #nosec
		if fileErr != nil {
			return err
		}
		defer func() { _ = src.Close() }()

		dst, err := os.OpenFile(t.tGoPath(path), os.O_RDWR|os.O_CREATE, info.Mode()|0200)
		if err != nil {
			return err
		}
		defer func() { _ = dst.Close() }()
		if _, err := io.Copy(dst, src); err != nil {
			return err
		}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(path)
		if err != nil {
			return err
		}
		if err := os.Symlink(link, t.tGoPath(path)); err != nil {
			return err
		}
		if err := os.Chmod(t.tGoPath(path), info.Mode()|0200); err != nil {
			return err
		}
	}
	return nil
}
