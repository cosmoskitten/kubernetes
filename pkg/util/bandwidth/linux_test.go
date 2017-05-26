// +build linux

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

package bandwidth

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kubernetes/pkg/util/exec"
)

var tcClassOutput = `class htb 1:1 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
class htb 1:2 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
class htb 1:3 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
class htb 1:4 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
`

var tcClassOutput2 = `class htb 1:1 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
class htb 1:2 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
class htb 1:3 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
class htb 1:4 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
class htb 1:5 root prio 0 rate 10000bit ceil 10000bit burst 1600b cburst 1600b 
`

func TestNextClassID(t *testing.T) {
	tests := []struct {
		output    string
		expectErr bool
		expected  int
		err       error
	}{
		{
			output:   tcClassOutput,
			expected: 5,
		},
		{
			output:   "\n",
			expected: 1,
		},
		{
			expected:  -1,
			expectErr: true,
			err:       errors.New("test error"),
		},
	}
	for _, test := range tests {
		fexec := exec.FakeExec{}
		fexec.ExpectCombinedOutput("tc class show dev cbr0", test.output, test.err)
		shaper := &tcShaper{e: &fexec, iface: "cbr0"}
		class, err := shaper.nextClassID()
		if test.expectErr {
			if err == nil {
				t.Errorf("unexpected non-error")
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if class != test.expected {
				t.Errorf("expected: %d, found %d", test.expected, class)
			}
		}
		fexec.AssertExpectedCommands()
	}
}

func TestHexCIDR(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		output    string
		expectErr bool
	}{
		{
			name:   "IPv4 masked",
			input:  "1.2.3.4/16",
			output: "01020000/ffff0000",
		},
		{
			name:   "IPv4 host",
			input:  "172.17.0.2/32",
			output: "ac110002/ffffffff",
		},
		{
			name:   "IPv6 masked",
			input:  "2001:dead:beef::cafe/64",
			output: "2001deadbeef00000000000000000000/ffffffffffffffff0000000000000000",
		},
		{
			name:   "IPv6 host",
			input:  "2001::5/128",
			output: "20010000000000000000000000000005/ffffffffffffffffffffffffffffffff",
		},
		{
			name:      "invalid CIDR",
			input:     "foo",
			expectErr: true,
		},
	}
	for _, test := range tests {
		output, err := hexCIDR(test.input)
		if test.expectErr {
			if err == nil {
				t.Errorf("case %s: unexpected non-error", test.name)
			}
		} else {
			if err != nil {
				t.Errorf("case %s: unexpected error: %v", test.name, err)
			}
			if output != test.output {
				t.Errorf("case %s: expected: %s, saw: %s",
					test.name, test.output, output)
			}
		}
	}
}

func TestAsciiCIDR(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		output    string
		expectErr bool
	}{
		{
			name:   "IPv4",
			input:  "01020000/ffff0000",
			output: "1.2.0.0/16",
		},
		{
			name:   "IPv4 host",
			input:  "ac110002/ffffffff",
			output: "172.17.0.2/32",
		},
		{
			name:   "IPv6",
			input:  "2001deadbeef00000000000000000000/ffffffffffffffff0000000000000000",
			output: "2001:dead:beef::/64",
		},
		{
			name:   "IPv6 host",
			input:  "20010000000000000000000000000005/ffffffffffffffffffffffffffffffff",
			output: "2001::5/128",
		},
		{
			name:      "invalid CIDR",
			input:     "malformed",
			expectErr: true,
		},
		{
			name:      "non-hex IP",
			input:     "nonhex/32",
			expectErr: true,
		},
		{
			name:      "non-hex mask",
			input:     "01020000/badmask",
			expectErr: true,
		},
	}
	for _, test := range tests {
		output, err := asciiCIDR(test.input)
		if test.expectErr {
			if err == nil {
				t.Errorf("case %s: unexpected non-error", test.name)
			}
		} else {
			if err != nil {
				t.Errorf("case %s: unexpected error: %v", test.name, err)
			}
			if output != test.output {
				t.Errorf("case %s: expected: %s, saw: %s",
					test.name, test.output, output)
			}
		}
	}
}

var tcFilterOutput = `filter parent 1: protocol ip pref 1 u32 
filter parent 1: protocol ip pref 1 u32 fh 800: ht divisor 1 
filter parent 1: protocol ip pref 1 u32 fh 800::800 order 2048 key ht 800 bkt 0 flowid 1:1 
  match ac110002/ffffffff at 16
filter parent 1: protocol ip pref 1 u32 fh 800::801 order 2049 key ht 800 bkt 0 flowid 1:2 
  match 01020000/ffff0000 at 16
`

func TestFindCIDRClass(t *testing.T) {
	tests := []struct {
		cidr           string
		output         string
		expectErr      bool
		expectNotFound bool
		expectedClass  string
		expectedHandle string
		err            error
	}{
		{
			cidr:           "172.17.0.2/32",
			output:         tcFilterOutput,
			expectedClass:  "1:1",
			expectedHandle: "800::800",
		},
		{
			cidr:           "1.2.3.4/16",
			output:         tcFilterOutput,
			expectedClass:  "1:2",
			expectedHandle: "800::801",
		},
		{
			cidr:           "2.2.3.4/16",
			output:         tcFilterOutput,
			expectNotFound: true,
		},
		{
			err:       errors.New("test error"),
			expectErr: true,
		},
	}
	for _, test := range tests {
		fexec := exec.FakeExec{}
		fexec.ExpectCombinedOutput("tc filter show dev cbr0", test.output, test.err)
		shaper := &tcShaper{e: &fexec, iface: "cbr0"}
		class, handle, found, err := shaper.findCIDRClass(test.cidr)
		if test.expectErr {
			if err == nil {
				t.Errorf("unexpected non-error")
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if test.expectNotFound {
				if found {
					t.Errorf("unexpectedly found an interface: %s %s", class, handle)
				}
			} else {
				if class != test.expectedClass {
					t.Errorf("expected: %s, found %s", test.expectedClass, class)
				}
				if handle != test.expectedHandle {
					t.Errorf("expected: %s, found %s", test.expectedHandle, handle)
				}
			}
		}
		fexec.AssertExpectedCommands()
	}
}

func TestGetCIDRs(t *testing.T) {
	fexec := exec.FakeExec{}
	fexec.ExpectCombinedOutput("tc filter show dev cbr0", tcFilterOutput, nil)
	shaper := &tcShaper{e: &fexec, iface: "cbr0"}
	cidrs, err := shaper.GetCIDRs()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	expectedCidrs := []string{"172.17.0.2/32", "1.2.0.0/16"}
	if !reflect.DeepEqual(cidrs, expectedCidrs) {
		t.Errorf("expected: %v, saw: %v", expectedCidrs, cidrs)
	}
	fexec.AssertExpectedCommands()
}

func TestLimit(t *testing.T) {
	tests := []struct {
		cidr          string
		ingress       *resource.Quantity
		egress        *resource.Quantity
		err           error
	}{
		{
			cidr:          "1.2.3.4/32",
			ingress:       resource.NewQuantity(10, resource.DecimalSI),
			egress:        resource.NewQuantity(20, resource.DecimalSI),
		},
		{
			cidr:          "1.2.3.4/32",
			ingress:       resource.NewQuantity(10, resource.DecimalSI),
			egress:        nil,
		},
		{
			cidr:          "1.2.3.4/32",
			ingress:       nil,
			egress:        resource.NewQuantity(20, resource.DecimalSI),
		},
		{
			cidr:          "1.2.3.4/32",
			ingress:       nil,
			egress:        nil,
		},
		{
			err:       errors.New("test error"),
			ingress:   resource.NewQuantity(10, resource.DecimalSI),
			egress:    resource.NewQuantity(20, resource.DecimalSI),
		},
	}

	for n, test := range tests {
		fexec := exec.FakeExec{}
		if test.err != nil {
			fexec.ExpectCombinedOutput("tc class show dev cbr0", "", test.err)
		} else {
			nextClassShowOutput := tcClassOutput
			nextClassID := "1:5"
			if test.egress != nil {
				fexec.ExpectCombinedOutput("tc class show dev cbr0", nextClassShowOutput, nil)
				fexec.ExpectCombinedOutput(fmt.Sprintf("tc class add dev cbr0 parent 1: classid %s htb rate %s", nextClassID, makeKBitString(test.egress)), "", nil)
				fexec.ExpectCombinedOutput(fmt.Sprintf("tc filter add dev cbr0 protocol ip parent 1:0 prio 1 u32 match ip dst %s flowid %s", test.cidr, nextClassID), "", nil)

				nextClassShowOutput = tcClassOutput2
				nextClassID = "1:6"
			}
			if test.ingress != nil {
				fexec.ExpectCombinedOutput("tc class show dev cbr0", nextClassShowOutput, nil)
				fexec.ExpectCombinedOutput(fmt.Sprintf("tc class add dev cbr0 parent 1: classid %s htb rate %s", nextClassID, makeKBitString(test.ingress)), "", nil)
				fexec.ExpectCombinedOutput(fmt.Sprintf("tc filter add dev cbr0 protocol ip parent 1:0 prio 1 u32 match ip src %s flowid %s", test.cidr, nextClassID), "", nil)
			}
		}
		shaper := &tcShaper{e: &fexec, iface: "cbr0"}
		if err := shaper.Limit(test.cidr, test.ingress, test.egress); err != nil && test.err == nil {
			t.Errorf("unexpected error on %d: %v", n, err)
			return
		} else if err == nil && test.err != nil {
			t.Error("unexpected non-error")
			return
		}
		fexec.AssertExpectedCommands()
	}
}

func TestReset(t *testing.T) {
	tests := []struct {
		cidr           string
		err            error
		expectedHandle string
		expectedClass  string
	}{
		{
			cidr:           "1.2.3.4/16",
			expectedHandle: "800::801",
			expectedClass:  "1:2",
		},
		{
			cidr:           "172.17.0.2/32",
			expectedHandle: "800::800",
			expectedClass:  "1:1",
		},
		{
			err:       errors.New("test error"),
		},
	}
	for _, test := range tests {
		fexec := exec.FakeExec{}
		if test.err != nil {
			fexec.ExpectCombinedOutput("tc filter show dev cbr0", "", test.err)
		} else {
			fexec.ExpectCombinedOutput("tc filter show dev cbr0", tcFilterOutput, nil)
			fexec.ExpectCombinedOutput(fmt.Sprintf("tc filter del dev cbr0 parent 1: proto ip prio 1 handle %s u32", test.expectedHandle), "", test.err)
			fexec.ExpectCombinedOutput(fmt.Sprintf("tc class del dev cbr0 parent 1: classid %s", test.expectedClass), "", test.err)
		}
		shaper := &tcShaper{e: &fexec, iface: "cbr0"}

		if err := shaper.Reset(test.cidr); err != nil && test.err == nil {
			t.Errorf("unexpected error: %v", err)
			return
		} else if test.err != nil && err == nil {
			t.Error("unexpected non-error")
			return
		}
		fexec.AssertExpectedCommands()
	}
}

var tcQdisc = "qdisc htb 1: root refcnt 2 r2q 10 default 30 direct_packets_stat 0\n"

func TestReconcileInterfaceExists(t *testing.T) {
	fexec := exec.FakeExec{}
	fexec.ExpectCombinedOutput("tc qdisc show dev cbr0", tcQdisc, nil)
	shaper := &tcShaper{e: &fexec, iface: "cbr0"}
	err := shaper.ReconcileInterface()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	fexec.AssertExpectedCommands()
}

func testReconcileInterfaceHasNoData(t *testing.T, output string) {
	fexec := exec.FakeExec{}
	fexec.ExpectCombinedOutput("tc qdisc show dev cbr0", output, nil)
	fexec.ExpectCombinedOutput("tc qdisc add dev cbr0 root handle 1: htb default 30", "", nil)
	shaper := &tcShaper{e: &fexec, iface: "cbr0"}
	err := shaper.ReconcileInterface()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	fexec.AssertExpectedCommands()
}

func TestReconcileInterfaceDoesntExist(t *testing.T) {
	testReconcileInterfaceHasNoData(t, "\n")
}

var tcQdiscNoqueue = "qdisc noqueue 0: root refcnt 2 \n"

func TestReconcileInterfaceExistsWithNoqueue(t *testing.T) {
	testReconcileInterfaceHasNoData(t, tcQdiscNoqueue)
}

func TestReconcileInterfaceIsWrong(t *testing.T) {
	fexec := exec.FakeExec{}
	fexec.ExpectCombinedOutput("tc qdisc show dev cbr0", "qdisc htb 2: root refcnt 2 r2q 10 default 30 direct_packets_stat 0\n", nil)
	fexec.ExpectCombinedOutput("tc qdisc delete dev cbr0 root handle 2:", "", nil)
	fexec.ExpectCombinedOutput("tc qdisc add dev cbr0 root handle 1: htb default 30", "", nil)
	shaper := &tcShaper{e: &fexec, iface: "cbr0"}
	err := shaper.ReconcileInterface()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	fexec.AssertExpectedCommands()

	fexec = exec.FakeExec{}
	fexec.ExpectCombinedOutput("tc qdisc show dev cbr0", "qdisc foo 1: root refcnt 2 r2q 10 default 30 direct_packets_stat 0\n", nil)
	fexec.ExpectCombinedOutput("tc qdisc delete dev cbr0 root handle 1:", "", nil)
	fexec.ExpectCombinedOutput("tc qdisc add dev cbr0 root handle 1: htb default 30", "", nil)
	shaper = &tcShaper{e: &fexec, iface: "cbr0"}
	err = shaper.ReconcileInterface()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	fexec.AssertExpectedCommands()
}
