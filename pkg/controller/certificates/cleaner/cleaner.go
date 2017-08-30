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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	certificatesinformers "k8s.io/client-go/informers/certificates/v1beta1"
	csrclient "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	certificateslisters "k8s.io/client-go/listers/certificates/v1beta1"
)

const (
	// The interval to list all CSRs and check each one against the criteria to
	// automatically clean it up.
	pollingInterval = 1 * time.Hour
	// The time periods after which these different CSR statuses should be
	// cleaned up.
	approvedExpiration = 1 * time.Hour
	deniedExpiration   = 1 * time.Hour
	pendingExpiration  = 24 * time.Hour
)

// CSRCleanerController is a controller that garbage collects old certificate
// signing requests (CSRs). Since there are mechanisms that automatically
// create CSRs, and mechanisms that automatically approve CSRs, in order to
// prevent a build up of CSRs over time, it is necessary to GC them.
type CSRCleanerController struct {
	csrClient csrclient.CertificateSigningRequestInterface
	csrLister certificateslisters.CertificateSigningRequestLister
}

// NewCSRCleanerController creates a new CSRCleanerController.
func NewCSRCleanerController(
	csrClient csrclient.CertificateSigningRequestInterface,
	csrInformer certificatesinformers.CertificateSigningRequestInformer,
) *CSRCleanerController {
	return &CSRCleanerController{
		csrClient: csrClient,
		csrLister: csrInformer.Lister(),
	}
}

// Run the main goroutine responsible for watching and syncing jobs.
func (ccc *CSRCleanerController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	glog.Infof("Starting CSR cleaner controller")
	defer glog.Infof("Shutting down CSR cleaner controller")

	for i := 0; i < workers; i++ {
		go wait.Until(ccc.worker, pollingInterval, stopCh)
	}

	<-stopCh
}

// worker runs a thread that dequeues CSRs, handles them, and marks them done.
func (ccc *CSRCleanerController) worker() {
	csrs, err := ccc.csrLister.List(labels.Everything())
	if err != nil {
		glog.Errorf("Unable to list CSRs: %v", err)
		return
	}
	for _, csr := range csrs {
		if err := ccc.handle(csr); err != nil {
			glog.Errorf("Error while attempting to clean CSR %q: %v", csr.Name, err)
		}
	}
}

func (ccc *CSRCleanerController) handle(csr *capi.CertificateSigningRequest) error {
	for _, c := range csr.Status.Conditions {
		if c.Type == capi.CertificateApproved && isIssued(csr) && isOlderThan(c.LastUpdateTime, approvedExpiration) {
			glog.Infof("Cleaning CSR %q as it more than %v old and approved.", csr.Name, approvedExpiration)
			if err := ccc.csrClient.Delete(csr.Name, nil); err != nil {
				return fmt.Errorf("unable to delete CSR %q: %v", csr.Name, err)
			}
		} else if c.Type == capi.CertificateDenied && isOlderThan(c.LastUpdateTime, deniedExpiration) {
			glog.Infof("Cleaning CSR %q as it more than %v old and denied.", csr.Name, deniedExpiration)
			if err := ccc.csrClient.Delete(csr.Name, nil); err != nil {
				return fmt.Errorf("unable to delete CSR %q: %v", csr.Name, err)
			}
			return nil
		}
	}
	// If there are no Conditions on the status, the CSR will appear via
	// `kubectl` as `Pending`.
	if len(csr.Status.Conditions) == 0 && isOlderThan(csr.CreationTimestamp, pendingExpiration) {
		glog.Infof("Cleaning CSR %q as it more than %v old and unhandled.", csr.Name, pendingExpiration)
		if err := ccc.csrClient.Delete(csr.Name, nil); err != nil {
			return fmt.Errorf("unable to delete CSR %q: %v", csr.Name, err)
		}
		return nil
	}

	return nil
}

func isOlderThan(t metav1.Time, d time.Duration) bool {
	return !t.IsZero() && t.Sub(time.Now()) < -1*d
}

// isIssued checks if the CSR has `Issued` status. There is no explicit
// 'Issued' status. Implicitly, if there is a certificate associated with the
// CSR, the CSR statuses that are visible via `kubectl` will include 'Issued'.
func isIssued(csr *capi.CertificateSigningRequest) bool {
	return csr.Status.Certificate != nil
}
