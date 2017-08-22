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

package dryrun

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"

	"k8s.io/apimachinery/pkg/util/errors"
)

// PrintBytesWithLinePrefix prints objBytes to writer w with linePrefix in the beginning of every line
func PrintBytesWithLinePrefix(w io.Writer, objBytes []byte, linePrefix string) {
	scanner := bufio.NewScanner(bytes.NewReader(objBytes))
	for scanner.Scan() {
		fmt.Fprintf(w, "%s%s\n", linePrefix, scanner.Text())
	}
}

// DryRunFile represents a temporary file on disk that might want to be aliased when printing
// Useful for things like loading a file from /tmp/ but saying to the user "Would write file foo to /etc/kubernetes/..."
type DryRunFile struct {
	RealPath  string
	PrintPath string
}

// NewDryRunFile makes a new instance of DryRunFile with the specified arguments
func NewDryRunFile(realPath, printPath string) DryRunFile {
	return DryRunFile{
		RealPath:  realPath,
		PrintPath: printPath,
	}
}

// DryRunPrintFileContents prints the contents of the DryRunFiles given to it to the writer w
func DryRunPrintFileContents(files []DryRunFile, w io.Writer) error {
	errs := []error{}
	for _, file := range files {
		if len(file.RealPath) == 0 {
			continue
		}

		fileBytes, err := ioutil.ReadFile(file.RealPath)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// Make it possible to fake the path of the file; i.e. you may want to tell the user
		// "Here is what would be written to /etc/kubernetes/admin.conf", although you wrote it to /tmp/kubeadm-dryrun/admin.conf and are loading it from there
		// Fall back to the "real" path if PrintPath is not set
		outputFilePath := file.PrintPath
		if len(outputFilePath) == 0 {
			outputFilePath = file.RealPath
		}

		fmt.Fprintf(w, "[dryrun] Would write file %q with content:\n", outputFilePath)
		PrintBytesWithLinePrefix(w, fileBytes, "\t")
	}
	return errors.NewAggregate(errs)
}
