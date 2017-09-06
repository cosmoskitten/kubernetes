/*
Copyright 2015 The Kubernetes Authors.

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

package util

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/utils/exec"
	fakeexec "k8s.io/utils/exec/testing"
)

func familyParamStr(isIPv6 bool) string {
	if isIPv6 {
		return " -f ipv6"
	}
	return ""
}

func TestExecConntrackTool(t *testing.T) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
			func() ([]byte, error) {
				return []byte(""), fmt.Errorf("conntrack v1.4.2 (conntrack-tools): 0 flow entries have been deleted")
			},
		},
	}
	fexec := fakeexec.FakeExec{
		CommandScript: []fakeexec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
		},
		LookPathFunc: func(cmd string) (string, error) { return cmd, nil },
	}

	testCases := [][]string{
		{"-L", "-p", "udp"},
		{"-D", "-p", "udp", "-d", "10.0.240.1"},
		{"-D", "-p", "udp", "--orig-dst", "10.240.0.2", "--dst-nat", "10.0.10.2"},
	}

	expectErr := []bool{false, false, true}

	for i := range testCases {
		err := ExecConntrackTool(&fexec, testCases[i]...)

		if expectErr[i] {
			if err == nil {
				t.Errorf("expected err, got %v", err)
			}
		} else {
			if err != nil {
				t.Errorf("expected success, got %v", err)
			}
		}

		execCmd := strings.Join(fcmd.CombinedOutputLog[i], " ")
		expectCmd := fmt.Sprintf("%s %s", "conntrack", strings.Join(testCases[i], " "))

		if execCmd != expectCmd {
			t.Errorf("expect execute command: %s, but got: %s", expectCmd, execCmd)
		}
	}
}

func TestClearUDPConntrackForIP(t *testing.T) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
			func() ([]byte, error) {
				return []byte(""), fmt.Errorf("conntrack v1.4.2 (conntrack-tools): 0 flow entries have been deleted")
			},
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
		},
	}
	fexec := fakeexec.FakeExec{
		CommandScript: []fakeexec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
		},
		LookPathFunc: func(cmd string) (string, error) { return cmd, nil },
	}

	ips := []string{
		"10.240.0.3",   // Success
		"10.240.0.5",   // Success
		"10.240.0.4",   // Simulated: 0 entries deleted
		"2001:db8::10", // Success
	}

	svcCount := 0
	for _, ip := range ips {
		if err := ClearUDPConntrackForIP(&fexec, ip); err != nil {
			t.Errorf("Unexepected error: %v", err)
		}
		expectCommand := fmt.Sprintf("conntrack -D --orig-dst %s -p udp", ip) + familyParamStr(isIPv6(ip))
		execCommand := strings.Join(fcmd.CombinedOutputLog[svcCount], " ")
		if expectCommand != execCommand {
			t.Errorf("Expect command: %s, but executed %s", expectCommand, execCommand)
		}
		svcCount++
	}
	if svcCount != fexec.CommandCalls {
		t.Errorf("Expect command executed %d times, but got %d", svcCount, fexec.CommandCalls)
	}
}

func TestClearUDPConntrackForPort(t *testing.T) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
			func() ([]byte, error) {
				return []byte(""), fmt.Errorf("conntrack v1.4.2 (conntrack-tools): 0 flow entries have been deleted")
			},
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
		},
	}
	fexec := fakeexec.FakeExec{
		CommandScript: []fakeexec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
		},
		LookPathFunc: func(cmd string) (string, error) { return cmd, nil },
	}

	testCases := []struct {
		port   int
		isIPv6 bool
	}{
		{8080, false},
		{9090, false},
		{6666, true},
	}
	svcCount := 0
	for _, tc := range testCases {
		err := ClearUDPConntrackForPort(&fexec, tc.port, tc.isIPv6)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		expectCommand := fmt.Sprintf("conntrack -D -p udp --dport %d", tc.port) + familyParamStr(tc.isIPv6)
		execCommand := strings.Join(fcmd.CombinedOutputLog[svcCount], " ")
		if expectCommand != execCommand {
			t.Errorf("Expect command: %s, but executed %s", expectCommand, execCommand)
		}
		svcCount++
	}
	if svcCount != fexec.CommandCalls {
		t.Errorf("Expect command executed %d times, but got %d", svcCount, fexec.CommandCalls)
	}
}

func TestDeleteUDPConnections(t *testing.T) {
	fcmd := fakeexec.FakeCmd{
		CombinedOutputScript: []fakeexec.FakeCombinedOutputAction{
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
			func() ([]byte, error) {
				return []byte(""), fmt.Errorf("conntrack v1.4.2 (conntrack-tools): 0 flow entries have been deleted")
			},
			func() ([]byte, error) { return []byte("1 flow entries have been deleted"), nil },
		},
	}
	fexec := fakeexec.FakeExec{
		CommandScript: []fakeexec.FakeCommandAction{
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
			func(cmd string, args ...string) exec.Cmd { return fakeexec.InitFakeCmd(&fcmd, cmd, args...) },
		},
		LookPathFunc: func(cmd string) (string, error) { return cmd, nil },
	}

	testCases := []struct {
		origin string
		dest   string
	}{
		{
			origin: "1.2.3.4",
			dest:   "10.20.30.40",
		},
		{
			origin: "2.3.4.5",
			dest:   "20.30.40.50",
		},
		{
			origin: "fd00::600d:f00d",
			dest:   "2001:db8::5",
		},
	}
	svcCount := 0
	for i, tc := range testCases {
		err := ClearUDPConntrackForPeers(&fexec, tc.origin, tc.dest)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		expectCommand := fmt.Sprintf("conntrack -D --orig-dst %s --dst-nat %s -p udp", tc.origin, tc.dest) + familyParamStr(isIPv6(tc.origin))
		execCommand := strings.Join(fcmd.CombinedOutputLog[i], " ")
		if expectCommand != execCommand {
			t.Errorf("Expect command: %s, but executed %s", expectCommand, execCommand)
		}
		svcCount++
	}
	if svcCount != fexec.CommandCalls {
		t.Errorf("Expect command executed %d times, but got %d", svcCount, fexec.CommandCalls)
	}
}
