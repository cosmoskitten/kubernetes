// +build linux

/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package host_path

import (
	"k8s.io/kubernetes/pkg/util/nsenter"
)

type nsenterFileTypeChecker struct {
	path   string
	exists bool
	ne     *nsenter.Nsenter
}

func newNsenterFileTypeChecker(path string) (hostPathTypeChecker, error) {
	ftc := &nsenterFileTypeChecker{
		path: path,
		ne:   nsenter.NewNsenter(),
	}
	ftc.Exists()
	return ftc, nil
}

func (ftc *nsenterFileTypeChecker) Exists() bool {
	args := append(ftc.ne.MakeBaseNsenterCmd("ls"), ftc.path)
	_, err := ftc.ne.Exec(args...).CombinedOutput()
	if err == nil {
		ftc.exists = true
	}
	return ftc.exists
}

func (ftc *nsenterFileTypeChecker) IsFile() bool {
	if !ftc.Exists() {
		return false
	}
	return !ftc.IsDir()
}

func (ftc *nsenterFileTypeChecker) MakeFile() error {
	args := append(ftc.ne.MakeBaseNsenterCmd("touch"), ftc.path)
	if _, err := ftc.ne.Exec(args...).CombinedOutput(); err != nil {
		return err
	}
	return nil
}

func (ftc *nsenterFileTypeChecker) IsDir() bool {
	return ftc.checkMimetype("directory")
}

func (ftc *nsenterFileTypeChecker) MakeDir() error {
	args := append(ftc.ne.MakeBaseNsenterCmd("mkdir"), []string{"-p", ftc.path}...)
	if _, err := ftc.ne.Exec(args...).CombinedOutput(); err != nil {
		return err
	}
	return nil
}

func (ftc *nsenterFileTypeChecker) IsBlock() bool {
	return ftc.checkMimetype("block special file")
}

func (ftc *nsenterFileTypeChecker) IsChar() bool {
	return ftc.checkMimetype("character special file")
}

func (ftc *nsenterFileTypeChecker) IsSocket() bool {
	return ftc.checkMimetype("socket")
}

func (ftc *nsenterFileTypeChecker) GetPath() string {
	return ftc.path
}

func (ftc *nsenterFileTypeChecker) checkMimetype(checkedType string) bool {
	if !ftc.Exists() {
		return false
	}
	args := append(ftc.ne.MakeBaseNsenterCmd("stat"),
		[]string{"-L", `--printf "%F"`, ftc.path}...)
	outputBytes, err := ftc.ne.Exec(args...).CombinedOutput()
	if err != nil {
		return false
	}
	return string(outputBytes) == checkedType
}
