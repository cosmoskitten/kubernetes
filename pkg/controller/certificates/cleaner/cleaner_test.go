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

package cleaner

import (
	"testing"
	"time"

	capi "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCleanerWithApprovedExpiredCSR(t *testing.T) {
	csr := &capi.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "TestCleanerWithApprovedExpiredCSR-fake-csr",
		},
		Status: capi.CertificateSigningRequestStatus{
			Certificate: []byte{0},
			Conditions: []capi.CertificateSigningRequestCondition{
				capi.CertificateSigningRequestCondition{
					Type:           capi.CertificateApproved,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
				},
			},
		},
	}

	client := fake.NewSimpleClientset(csr)
	s := &csrCleaner{
		client: client,
	}

	err := s.handle(csr)
	if err != nil {
		t.Fatalf("failed to clean CSR: %v", err)
	}

	actions := client.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 actions")
	}
	if a := actions[0]; !a.Matches("delete", "certificatesigningrequests") {
		t.Errorf("unexpected action: %#v", a)
	}
}

func TestCleanerWithApprovedUnexpiredCSR(t *testing.T) {
	csr := &capi.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "TestCleanerWithApprovedUnexpiredCSR-fake-csr",
		},
		Status: capi.CertificateSigningRequestStatus{
			Certificate: []byte{0},
			Conditions: []capi.CertificateSigningRequestCondition{
				capi.CertificateSigningRequestCondition{
					Type:           capi.CertificateApproved,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-50 * time.Minute)),
				},
			},
		},
	}

	client := fake.NewSimpleClientset(csr)
	s := &csrCleaner{
		client: client,
	}

	err := s.handle(csr)
	if err != nil {
		t.Fatalf("failed to clean CSR: %v", err)
	}

	actions := client.Actions()
	if len(actions) != 0 {
		t.Errorf("expected 0 actions")
		for _, a := range actions {
			t.Errorf("  verb = %q, resource = %q", a.GetVerb(), a.GetResource())
		}
	}
}

func TestCleanerWithDeniedExpiredCSR(t *testing.T) {
	csr := &capi.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "TestCleanerWithDeniedExpiredCSR-fake-csr",
		},
		Status: capi.CertificateSigningRequestStatus{
			Certificate: []byte{0},
			Conditions: []capi.CertificateSigningRequestCondition{
				capi.CertificateSigningRequestCondition{
					Type:           capi.CertificateDenied,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
				},
			},
		},
	}

	client := fake.NewSimpleClientset(csr)
	s := &csrCleaner{
		client: client,
	}

	err := s.handle(csr)
	if err != nil {
		t.Fatalf("failed to clean CSR: %v", err)
	}

	actions := client.Actions()
	if len(actions) != 1 {
		t.Fatalf("expected 1 actions")
	}
	if a := actions[0]; !a.Matches("delete", "certificatesigningrequests") {
		t.Errorf("unexpected action: %#v", a)
	}
}

func TestCleanerWithDeniedUnexpiredCSR(t *testing.T) {
	csr := &capi.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "TestCleanerWithDeniedUnexpiredCSR-fake-csr",
		},
		Status: capi.CertificateSigningRequestStatus{
			Certificate: []byte{0},
			Conditions: []capi.CertificateSigningRequestCondition{
				capi.CertificateSigningRequestCondition{
					Type:           capi.CertificateDenied,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-50 * time.Minute)),
				},
			},
		},
	}

	client := fake.NewSimpleClientset(csr)
	s := &csrCleaner{
		client: client,
	}

	err := s.handle(csr)
	if err != nil {
		t.Fatalf("failed to clean CSR: %v", err)
	}

	actions := client.Actions()
	if len(actions) != 0 {
		t.Errorf("expected 0 actions")
		for _, a := range actions {
			t.Errorf("  verb = %q, resource = %q", a.GetVerb(), a.GetResource())
		}
	}
}
