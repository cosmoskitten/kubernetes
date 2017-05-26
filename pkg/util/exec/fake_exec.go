/*
Copyright 2014 The Kubernetes Authors.

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

package exec

import (
	"fmt"
	"io"
	"reflect"
	"strings"
)

// A simple scripted Interface type.
type FakeExec struct {
	CommandScript []FakeCommandAction
	CommandCalls  int
	LookPathFunc  func(string) (string, error)
}

type FakeCommandAction func(cmd string, args ...string) Cmd

func (fake *FakeExec) Command(cmd string, args ...string) Cmd {
	if fake.CommandCalls > len(fake.CommandScript)-1 {
		panic(fmt.Sprintf("ran out of Command() actions. Could not handle command [%d]: %s args: %v", fake.CommandCalls, cmd, args))
	}
	i := fake.CommandCalls
	fake.CommandCalls++
	return fake.CommandScript[i](cmd, args...)
}

func (fake *FakeExec) LookPath(file string) (string, error) {
	return fake.LookPathFunc(file)
}

// A simple scripted Cmd type.
type FakeCmd struct {
	Argv                 []string
	CombinedOutputScript []FakeCombinedOutputAction
	CombinedOutputCalls  int
	CombinedOutputLog    [][]string
	RunScript            []FakeRunAction
	RunCalls             int
	RunLog               [][]string
	Dirs                 []string
	Stdin                io.Reader
	Stdout               io.Writer
	Stderr               io.Writer
}

func InitFakeCmd(fake *FakeCmd, cmd string, args ...string) Cmd {
	fake.Argv = append([]string{cmd}, args...)
	return fake
}

type FakeCombinedOutputAction func() ([]byte, error)
type FakeRunAction func() ([]byte, []byte, error)

func (fake *FakeCmd) SetDir(dir string) {
	fake.Dirs = append(fake.Dirs, dir)
}

func (fake *FakeCmd) SetStdin(in io.Reader) {
	fake.Stdin = in
}

func (fake *FakeCmd) SetStdout(out io.Writer) {
	fake.Stdout = out
}

func (fake *FakeCmd) SetStderr(out io.Writer) {
	fake.Stderr = out
}

func (fake *FakeCmd) Run() error {
	if fake.RunCalls > len(fake.RunScript)-1 {
		panic("ran out of Run() actions")
	}
	if fake.RunLog == nil {
		fake.RunLog = [][]string{}
	}
	i := fake.RunCalls
	fake.RunLog = append(fake.RunLog, append([]string{}, fake.Argv...))
	fake.RunCalls++
	stdout, stderr, err := fake.RunScript[i]()
	if stdout != nil {
		fake.Stdout.Write(stdout)
	}
	if stderr != nil {
		fake.Stderr.Write(stderr)
	}
	return err
}

func (fake *FakeCmd) CombinedOutput() ([]byte, error) {
	if fake.CombinedOutputCalls > len(fake.CombinedOutputScript)-1 {
		panic("ran out of CombinedOutput() actions")
	}
	if fake.CombinedOutputLog == nil {
		fake.CombinedOutputLog = [][]string{}
	}
	i := fake.CombinedOutputCalls
	fake.CombinedOutputLog = append(fake.CombinedOutputLog, append([]string{}, fake.Argv...))
	fake.CombinedOutputCalls++
	return fake.CombinedOutputScript[i]()
}

func (fake *FakeCmd) Output() ([]byte, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (fake *FakeCmd) Stop() {
	// no-op
}

// A simple fake ExitError type.
type FakeExitError struct {
	Status int
}

func (fake *FakeExitError) String() string {
	return fmt.Sprintf("exit %d", fake.Status)
}

func (fake *FakeExitError) Error() string {
	return fake.String()
}

func (fake *FakeExitError) Exited() bool {
	return true
}

func (fake *FakeExitError) ExitStatus() int {
	return fake.Status
}

func commandAsStringSlice(command interface{}) []string {
	if args, ok := command.([]string); ok {
		return args
	} else if commandstr, ok := command.(string); ok {
		return strings.Split(commandstr, " ")
	}
	panic("command must be []string or string")
}

func outputAsBytes(output interface{}) []byte {
	if outputBytes, ok := output.([]byte); ok {
		return outputBytes
	} else if outputStr, ok := output.(string); ok {
		return []byte(outputStr)
	} else if output == nil {
		return nil
	}
	panic("output must be []byte or string")
}

func (fake *FakeExec) expectFakeCmd(command interface{}, fcmd *FakeCmd) {
	expectedArgv := commandAsStringSlice(command)
	fake.CommandScript = append(fake.CommandScript,
		func(cmd string, args ...string) Cmd {
			InitFakeCmd(fcmd, cmd, args...)
			if !reflect.DeepEqual(expectedArgv, fcmd.Argv) {
				panic(fmt.Sprintf("Wrong Exec: expected %v got %v", expectedArgv, fcmd.Argv))
			}
			return fcmd
		},
	)
}

// ExpectCombinedOutput "predicts" a call to fake.Command(...).CombinedOutput() with the
// given command, and provides the result of that call.
// command is either a string (command line) or a []string (argv) indicating the expected command.
// output and err will be returned as the result of CombinedOutput() on the command.
// output can be either a []byte or a string (which will be converted to a []byte).
func (fake *FakeExec) ExpectCombinedOutput(command interface{}, output interface{}, err error) {
	fake.expectFakeCmd(command, &FakeCmd{
		CombinedOutputScript: []FakeCombinedOutputAction{
			func() ([]byte, error) { return outputAsBytes(output), err },
		},
	})
}

// ExpectRun "predicts" a call to fake.Command(...).Run() with the
// given command, and provides the result of that call.
// command is either a string (command line) or a []string (argv) indicating the expected command.
// stdout, stderr, and err will be returned as the result of Run() on the command.
// stdout and stderr can be either a []byte or a string (which will be converted to a []byte).
func (fake *FakeExec) ExpectRun(command interface{}, stdout interface{}, stderr interface{}, err error) {
	fake.expectFakeCmd(command, &FakeCmd{
		RunScript: []FakeRunAction{
			func() ([]byte, []byte, error) { return outputAsBytes(stdout), outputAsBytes(stderr), err },
		},
	})
}

// AssertExpectedCommands ensures that all of the commands added to fake via ExpectCombinedOutput and ExpectRun were actually executed
func (fake *FakeExec) AssertExpectedCommands() {
	if fake.CommandCalls != len(fake.CommandScript) {
		panic(fmt.Sprintf("Only used %d of %d expected commands", fake.CommandCalls, len(fake.CommandScript)))
	}
}
