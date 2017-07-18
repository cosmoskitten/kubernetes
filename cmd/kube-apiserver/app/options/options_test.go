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

package options

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

type Options struct {
	// --advertise-address
	AdvertiseAddress net.IP
	// --allow-privileged
	AllowPrivileged bool
	// --anonymous-auth
	AnonymousAuth bool
	// --apiserver-count
	ApiserverCount int
	// --audit-log-maxage
	AuditLogMaxage int
	// --audit-log-maxbackup
	AuditLogMaxbackup int
	// --audit-log-maxsize
	AuditLogMaxsize int
	// --audit-log-path
	AuditLogPath string
	// --audit-policy-file
	AuditPolicyFile string
	// --audit-webhook-config-file
	AuditWebhookConfigFile string
	// --audit-webhook-mode
	AuditWebhookMode string
	// --authentication-token-webhook-cache-ttl
	AuthenticationTokenWebhookCacheTTL time.Duration
	// --authentication-token-webhook-config-file
	AuthenticationTokenWebhookConfigFile string
	// --authorization-mode
	AuthorizationMode string
	// --authorization-policy-file
	AuthorizationPolicyFile string
	// --authorization-webhook-cache-authorized-ttl
	AuthorizationWebhookCacheAuthorizedTTL time.Duration
	// --authorization-webhook-cache-unauthorized-ttl
	AuthorizationWebhookCacheUnauthorizedTTL time.Duration
	// --authorization-webhook-config-file
	AuthorizationWebhookConfigFile string
	// --bind-address
	BindAddress net.IP
	// --client-ca-file
	ClientCAFile string
	// --cloud-config
	CloudConfig string
	// --cloud-provider
	CloudProvider string
	// --cors-allowed-origins
	CorsAllowedOrigins []string
	// --enable-aggregator-routing
	EnableAggregatorRouting bool
	// --enable-logs-handler
	EnableLogsHandler bool
	// --enable-swagger-ui
	EnableSwaggerUI bool
}

func TestAddFlagsFlag(t *testing.T) {
	// TODO: Expand the test to include other flags as well.
	f := pflag.NewFlagSet("addflagstest", pflag.ContinueOnError)
	s := NewServerRunOptions()
	s.AddFlags(f)

	args := []string{
		"--advertise-address=192.168.10.10",
		"--allow-privileged=false",
		"--anonymous-auth=false",
		"--apiserver-count=5",
		"--audit-log-maxage=11",
		"--audit-log-maxbackup=12",
		"--audit-log-maxsize=13",
		"--audit-log-path=/var/log",
		"--audit-policy-file=/policy",
		"--audit-webhook-config-file=/webhook-config",
		"--audit-webhook-mode=blocking",
		"--authentication-token-webhook-cache-ttl=3m",
		"--authentication-token-webhook-config-file=/token-webhook-config",
		"--authorization-mode=AlwaysDeny",
		"--authorization-policy-file=/policy",
		"--authorization-webhook-cache-authorized-ttl=3m",
		"--authorization-webhook-cache-unauthorized-ttl=1m",
		"--authorization-webhook-config-file=/webhook-config",
		"--bind-address=192.168.10.20",
		"--client-ca-file=/client-ca",
		"--cloud-config=/cloud-config",
		"--cloud-provider=daocloud",
		"--cors-allowed-origins=10.10.10.100,10.10.10.200",
		"--enable-aggregator-routing=true",
		"--enable-logs-handler=false",
		"--enable-swagger-ui=true",
	}
	f.Parse(args)

	// Values parsed by args
	parsedOps := Options{}
	parsedOps.AdvertiseAddress = s.GenericServerRunOptions.AdvertiseAddress
	parsedOps.AllowPrivileged = s.AllowPrivileged
	parsedOps.AnonymousAuth = s.Authentication.Anonymous.Allow
	parsedOps.ApiserverCount = s.MasterCount
	parsedOps.AuditLogMaxage = s.Audit.LogOptions.MaxAge
	parsedOps.AuditLogMaxbackup = s.Audit.LogOptions.MaxBackups
	parsedOps.AuditLogMaxsize = s.Audit.LogOptions.MaxSize
	parsedOps.AuditLogPath = s.Audit.LogOptions.Path
	parsedOps.AuditPolicyFile = s.Audit.PolicyFile
	parsedOps.AuditWebhookConfigFile = s.Audit.WebhookOptions.ConfigFile
	parsedOps.AuditWebhookMode = s.Audit.WebhookOptions.Mode
	parsedOps.AuthenticationTokenWebhookCacheTTL = s.Authentication.WebHook.CacheTTL
	parsedOps.AuthorizationWebhookConfigFile = s.Authentication.WebHook.ConfigFile
	parsedOps.AuthorizationMode = s.Authorization.Mode
	parsedOps.AuthorizationPolicyFile = s.Authorization.PolicyFile
	parsedOps.AuthorizationWebhookCacheAuthorizedTTL = s.Authorization.WebhookCacheAuthorizedTTL
	parsedOps.AuthorizationWebhookCacheUnauthorizedTTL = s.Authorization.WebhookCacheUnauthorizedTTL
	parsedOps.AuthorizationWebhookConfigFile = s.Authorization.WebhookConfigFile
	parsedOps.BindAddress = s.SecureServing.BindAddress
	parsedOps.ClientCAFile = s.Authentication.ClientCert.ClientCA
	parsedOps.CloudConfig = s.CloudProvider.CloudConfigFile
	parsedOps.CloudProvider = s.CloudProvider.CloudProvider
	parsedOps.CorsAllowedOrigins = s.GenericServerRunOptions.CorsAllowedOriginList
	parsedOps.EnableAggregatorRouting = s.EnableAggregatorRouting
	parsedOps.EnableLogsHandler = s.EnableLogsHandler
	parsedOps.EnableSwaggerUI = s.Features.EnableSwaggerUI

	// Values which parsed values should be equal to
	testOps := Options{}
	testOps.AdvertiseAddress = net.ParseIP("192.168.10.10")
	testOps.AllowPrivileged = false
	testOps.AnonymousAuth = false
	testOps.ApiserverCount = 5
	testOps.AuditLogMaxage = 11
	testOps.AuditLogMaxbackup = 12
	testOps.AuditLogMaxsize = 13
	testOps.AuditLogPath = "/var/log"
	testOps.AuditPolicyFile = "/policy"
	testOps.AuditWebhookConfigFile = "/webhook-config"
	testOps.AuditWebhookMode = "blocking"
	testOps.AuthenticationTokenWebhookCacheTTL = 180000000000
	testOps.AuthorizationWebhookConfigFile = "/token-webhook-config"
	testOps.AuthorizationMode = "AlwaysDeny"
	testOps.AuthorizationPolicyFile = "/policy"
	testOps.AuthorizationWebhookCacheAuthorizedTTL = 180000000000
	testOps.AuthorizationWebhookCacheUnauthorizedTTL = 60000000000
	testOps.AuthorizationWebhookConfigFile = "/webhook-config"
	testOps.BindAddress = net.ParseIP("192.168.10.20")
	testOps.ClientCAFile = "/client-ca"
	testOps.CloudConfig = "/cloud-config"
	testOps.CloudProvider = "daocloud"
	testOps.CorsAllowedOrigins = []string{"10.10.10.100", "10.10.10.200"}
	testOps.EnableAggregatorRouting = true
	testOps.EnableLogsHandler = false
	testOps.EnableSwaggerUI = true

	if !reflect.DeepEqual(parsedOps, testOps) {
		t.Errorf("Expected parsedOps %p to be equal to testOps %p", parsedOps, testOps)
	}
}
