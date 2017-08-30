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
	testCases := []struct {
		name            string
		created         metav1.Time
		certificate     []byte
		conditions      []capi.CertificateSigningRequestCondition
		expectedActions []string
	}{
		{
			"no delete approved unexpired",
			metav1.NewTime(time.Now().Add(-1 * time.Minute)),
			[]byte{0},
			[]capi.CertificateSigningRequestCondition{
				{
					Type:           capi.CertificateApproved,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-50 * time.Minute)),
				},
			},
			[]string{},
		},
		{
			"no delete approved expired not issued",
			metav1.NewTime(time.Now().Add(-1 * time.Minute)),
			nil,
			[]capi.CertificateSigningRequestCondition{
				{
					Type:           capi.CertificateApproved,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-50 * time.Minute)),
				},
			},
			[]string{},
		},
		{
			"delete approved expired",
			metav1.NewTime(time.Now().Add(-1 * time.Minute)),
			[]byte{0},
			[]capi.CertificateSigningRequestCondition{
				{
					Type:           capi.CertificateApproved,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
				},
			},
			[]string{"delete"},
		},
		{
			"no delete denied unexpired",
			metav1.NewTime(time.Now().Add(-1 * time.Minute)),
			nil,
			[]capi.CertificateSigningRequestCondition{
				{
					Type:           capi.CertificateDenied,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-50 * time.Minute)),
				},
			},
			[]string{},
		},
		{
			"delete denied expired",
			metav1.NewTime(time.Now().Add(-1 * time.Minute)),
			nil,
			[]capi.CertificateSigningRequestCondition{
				{
					Type:           capi.CertificateDenied,
					LastUpdateTime: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
				},
			},
			[]string{"delete"},
		},
		{
			"no delete pending unexpired",
			metav1.NewTime(time.Now().Add(-5 * time.Hour)),
			nil,
			[]capi.CertificateSigningRequestCondition{},
			[]string{},
		},
		{
			"delete pending expired",
			metav1.NewTime(time.Now().Add(-25 * time.Hour)),
			nil,
			[]capi.CertificateSigningRequestCondition{},
			[]string{"delete"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			csr := &capi.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "fake-csr",
					CreationTimestamp: tc.created,
				},
				Status: capi.CertificateSigningRequestStatus{
					Certificate: tc.certificate,
					Conditions:  tc.conditions,
				},
			}

			client := fake.NewSimpleClientset(csr)
			s := &CSRCleanerController{
				csrClient: client.CertificatesV1beta1().CertificateSigningRequests(),
			}

			err := s.handle(csr)
			if err != nil {
				t.Fatalf("failed to clean CSR: %v", err)
			}

			actions := client.Actions()
			if len(actions) != len(tc.expectedActions) {
				t.Fatalf("got %d actions, wanted %d actions", len(actions), len(tc.expectedActions))
			}
			for i := 0; i < len(actions); i++ {
				if a := actions[i]; !a.Matches(tc.expectedActions[i], "certificatesigningrequests") {
					t.Errorf("got action %#v, wanted %v", a, tc.expectedActions[i])
				}
			}
		})
	}
}
