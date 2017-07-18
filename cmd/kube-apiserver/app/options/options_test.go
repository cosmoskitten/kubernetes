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

	// Kubernetes apiserver run options
	parsed := Options{
		s.GenericServerRunOptions.AdvertiseAddress,
		s.AllowPrivileged,
		s.Authentication.Anonymous.Allow,
		s.MasterCount,
		s.Audit.LogOptions.MaxAge,
		s.Audit.LogOptions.MaxBackups,
		s.Audit.LogOptions.MaxSize,
		s.Audit.LogOptions.Path,
		s.Audit.PolicyFile,
		s.Audit.WebhookOptions.ConfigFile,
		s.Audit.WebhookOptions.Mode,
		s.Authentication.WebHook.CacheTTL,
		s.Authentication.WebHook.ConfigFile,
		s.Authorization.Mode,
		s.Authorization.PolicyFile,
		s.Authorization.WebhookCacheAuthorizedTTL,
		s.Authorization.WebhookCacheUnauthorizedTTL,
		s.Authorization.WebhookConfigFile,
		s.SecureServing.BindAddress,
		s.Authentication.ClientCert.ClientCA,
		s.CloudProvider.CloudConfigFile,
		s.CloudProvider.CloudProvider,
		s.GenericServerRunOptions.CorsAllowedOriginList,
		s.EnableAggregatorRouting,
		s.EnableLogsHandler,
		s.Features.EnableSwaggerUI,
	}

	// Expected apiserver run options set by args
	expected := Options{
		net.ParseIP("192.168.10.10"),
		false,
		false,
		5,
		11,
		12,
		13,
		"/var/log",
		"/policy",
		"/webhook-config",
		"blocking",
		180000000000,
		"/token-webhook-config",
		"AlwaysDeny",
		"/policy",
		180000000000,
		60000000000,
		"/webhook-config",
		net.ParseIP("192.168.10.20"),
		"/client-ca",
		"/cloud-config",
		"daocloud",
		[]string{"10.10.10.100", "10.10.10.200"},
		true,
		false,
		true,
	}

	if !reflect.DeepEqual(parsed, expected) {
		t.Logf("expected %s, got %s", expected, parsed)
		t.Errorf("Got different run options than expected.")
	}
}
