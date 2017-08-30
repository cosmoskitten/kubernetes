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

// Package cleaner implements an automated cleaner that does garbage collection
// on CSRs that meet specific criteria. With automated CSR requests and
// automated approvals, the volume of CSRs only increases over time, at a rapid
// rate if the certificate duration is short.
package cleaner

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	capi "k8s.io/api/certificates/v1beta1"
	certificatesinformers "k8s.io/client-go/informers/certificates/v1beta1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

const cleaningDelay = 1 * time.Hour

type csrCleaner struct {
	client clientset.Interface
}

func NewCSRCleanerController(
	client clientset.Interface,
	csrInformer certificatesinformers.CertificateSigningRequestInformer,
) (*certificates.CertificateController, error) {
	cleaner := &csrCleaner{
		client: client,
	}

	return certificates.NewCertificateControllerWithDelay(
		client,
		csrInformer,
		cleaner.handle,
		cleaningDelay,
	)
}

func (cc *csrCleaner) handle(csr *capi.CertificateSigningRequest) error {
	for _, c := range csr.Status.Conditions {
		switch c.Type {
		case capi.CertificateApproved:
			// There is no explicit 'Issued' status. Implicitly, if there is a
			// certificate associated with the CSR, the CSR statuses includes
			// 'Issued'.
			if !c.LastUpdateTime.IsZero() && c.LastUpdateTime.Sub(time.Now()) < -1*cleaningDelay && csr.Status.Certificate != nil {
				glog.Infof("Cleaning CSR %q as it more than %v after approval.", csr.Name, cleaningDelay)
				if err := cc.client.CertificatesV1beta1().CertificateSigningRequests().Delete(csr.Name, nil); err != nil {
					return fmt.Errorf("unable to delete CSR %q: %v", csr.Name, err)
				}
			}
		case capi.CertificateDenied:
			if !c.LastUpdateTime.IsZero() && c.LastUpdateTime.Sub(time.Now()) < -1*cleaningDelay {
				glog.Infof("Cleaning CSR %q as it more than %v after denial.", csr.Name, cleaningDelay)
				if err := cc.client.CertificatesV1beta1().CertificateSigningRequests().Delete(csr.Name, nil); err != nil {
					return fmt.Errorf("unable to delete CSR %q: %v", csr.Name, err)
				}
				return nil
			}
		}
	}

	return nil
}
