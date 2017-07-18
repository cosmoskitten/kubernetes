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
	"testing"

	"github.com/spf13/pflag"
)

func TestAddFlagsFlag(t *testing.T) {
	// TODO: This only tests the enable-swagger-ui flag for now.
	// Expand the test to include other flags as well.
	f := pflag.NewFlagSet("addflagstest", pflag.ContinueOnError)
	s := NewServerRunOptions()
	s.AddFlags(f)
	if s.GenericServerRunOptions.AdvertiseAddress != nil {
		t.Errorf("Expected s.GenericServerRunOptions.AdvertiseAddress to be nil by default")
	}
	if s.AllowPrivileged {
		t.Errorf("Expected s.AllowPrivileged to be false by default")
	}
	if !s.Authentication.Anonymous.Allow {
		t.Errorf("Expected s.Authentication.Anonymous.Allow to be true by default")
	}
	if s.MasterCount != 1 {
		t.Errorf("Expected s.MasterCount to be 1 by default")
	}
	if s.Audit.LogOptions.MaxAge != 0 {
		t.Errorf("Expected s.Audit.LogOptions.MaxAge to be 0 by default")
	}
	if s.Audit.LogOptions.MaxBackups != 0 {
		t.Errorf("Expected s.Audit.LogOptions.MaxBackups to be 0 by default")
	}
	if s.Audit.LogOptions.MaxSize != 0 {
		t.Errorf("Expected s.Audit.LogOptions.MaxSize to be 0 by default")
	}
	if s.Audit.LogOptions.Path != "" {
		t.Errorf("Expected s.Audit.LogOptions.Path to be empty by default")
	}
	if s.Audit.PolicyFile != "" {
		t.Errorf("Expected s.Audit.PolicyFile to be empty by default")
	}
	if s.Audit.WebhookOptions.ConfigFile != "" {
		t.Errorf("Expected s.Audit.WebhookOptions.ConfigFile to be empty by default")
	}
	if s.Audit.WebhookOptions.Mode != "batch" {
		t.Errorf("Expected s.Audit.WebhookOptions.Mode to be batch by default")
	}
	if s.Authentication.WebHook.CacheTTL != 120000000000 {
		t.Errorf("Expected s.Authentication.WebHook.CacheTTL to be 120000000000 by default")
	}
	if s.Authentication.WebHook.ConfigFile != "" {
		t.Errorf("Expected s.Authentication.WebHook.ConfigFile to be empty by default")
	}
	if s.Authorization.Mode != "AlwaysAllow" {
		t.Errorf("Expected s.Authorization.Mode to be AlwaysAllow by default")
	}
	if s.Authorization.PolicyFile != "" {
		t.Errorf("Expected s.Authorization.PolicyFile to be empty by default")
	}
	if s.Authorization.WebhookCacheAuthorizedTTL != 300000000000 {
		t.Errorf("Expected s.Authorization.WebhookCacheAuthorizedTTL to be 300000000000 by default")
	}
	if s.Authorization.WebhookCacheUnauthorizedTTL != 30000000000 {
		t.Errorf("Expected s.Authorization.WebhookCacheUnauthorizedTTL to be 30000000000 by default")
	}
	if s.Authorization.WebhookConfigFile != "" {
		t.Errorf("Expected s.Authorization.WebhookConfigFile to be empty by default")
	}
	if s.SecureServing.BindAddress.String() != "0.0.0.0" {
		t.Errorf("Expected s.SecureServing.BindAddress to be 0.0.0.0 by default")
	}
	if s.Authentication.ClientCert.ClientCA != "" {
		t.Errorf("Expected s.Authentication.ClientCert.ClientCA to be empty by default")
	}
	if s.CloudProvider.CloudConfigFile != "" {
		t.Errorf("Expected s.CloudProvider.CloudConfigFile to be empty by default")
	}
	if s.CloudProvider.CloudProvider != "" {
		t.Errorf("Expected s.CloudProvider.CloudProvider to be empty by default")
	}
	if s.GenericServerRunOptions.CorsAllowedOriginList != nil {
		t.Errorf("Expected s.GenericServerRunOptions.CorsAllowedOriginList to be empty by default")
	}
	if s.EnableAggregatorRouting {
		t.Errorf("Expected s.EnableAggregatorRouting to be false by default")
	}
	if !s.EnableLogsHandler {
		t.Errorf("Expected s.EnableLogsHandler to be true by default")
	}
	if s.Features.EnableSwaggerUI {
		t.Errorf("Expected s.Features.EnableSwaggerUI to be false by default")
	}

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

	// Check option --advertise-address
	if s.GenericServerRunOptions.AdvertiseAddress.String() != "192.168.10.10" {
		t.Errorf("Expected s.GenericServerRunOptions.AdvertiseAddress to be 192.168.10.10")
	}
	// Check option --allow-privileged
	if s.AllowPrivileged {
		t.Errorf("Expected s.AllowPrivileged to be false")
	}
	// Check option --anonymous-auth
	if s.Authentication.Anonymous.Allow {
		t.Errorf("Expected s.Authentication.Anonymous.Allow to be false")
	}
	// Check option --apiserver-coun
	if s.MasterCount != 5 {
		t.Errorf("Expected s.MasterCount to be 5")
	}
	// Check option --audit-log-maxage
	if s.Audit.LogOptions.MaxAge != 11 {
		t.Errorf("Expected s.Audit.LogOptions.MaxAge to be 11")
	}
	// Check option --audit-log-maxbackup
	if s.Audit.LogOptions.MaxBackups != 12 {
		t.Errorf("Expected s.Audit.LogOptions.MaxBackups to be 12")
	}
	// Check option --audit-log-maxsize
	if s.Audit.LogOptions.MaxSize != 13 {
		t.Errorf("Expected s.Audit.LogOptions.MaxSize to be 13")
	}
	// Check option --audit-log-path
	if s.Audit.LogOptions.Path != "/var/log" {
		t.Errorf("Expected s.Audit.LogOptions.Path to be /var/log")
	}
	// Check option --audit-policy-file
	if s.Audit.PolicyFile != "/policy" {
		t.Errorf("Expected s.Audit.PolicyFile to be /policy")
	}
	// Check option --audit-webhook-config-file
	if s.Audit.WebhookOptions.ConfigFile != "/webhook-config" {
		t.Errorf("Expected s.Audit.WebhookOptions.ConfigFile to be /webhook-config")
	}
	// Check option --audit-webhook-mode
	if s.Audit.WebhookOptions.Mode != "blocking" {
		t.Errorf("Expected s.Audit.WebhookOptions.Mode to be blocking")
	}
	// Check option --authentication-token-webhook-cache-ttl
	if s.Authentication.WebHook.CacheTTL != 180000000000 {
		t.Errorf("Expected s.Authentication.WebHook.CacheTTL to be 180000000000")
	}
	// Check option --authentication-token-webhook-config-file
	if s.Authentication.WebHook.ConfigFile != "/token-webhook-config" {
		t.Errorf("Expected s.Authentication.WebHook.ConfigFile to be /token-webhook-config")
	}
	// Check option --authorization-mode
	if s.Authorization.Mode != "AlwaysDeny" {
		t.Errorf("Expected s.Authorization.Mode to be AlwaysDeny")
	}
	// Check option --authorization-policy-file
	if s.Authorization.PolicyFile != "/policy" {
		t.Errorf("Expected s.Authorization.PolicyFile to be /policy")
	}
	// Check option --authorization-webhook-cache-authorized-ttl
	if s.Authorization.WebhookCacheAuthorizedTTL != 180000000000 {
		t.Errorf("Expected s.Authorization.WebhookCacheAuthorizedTTL to be 180000000000")
	}
	// Check option --authorization-webhook-cache-unauthorized-ttl
	if s.Authorization.WebhookCacheUnauthorizedTTL != 60000000000 {
		t.Errorf("Expected s.Authorization.WebhookCacheUnauthorizedTTL to be 60000000000")
	}
	// Check option --authorization-webhook-config-file
	if s.Authorization.WebhookConfigFile != "/webhook-config" {
		t.Errorf("Expected s.Authorization.WebhookConfigFile to be /webhook-config")
	}
	// Check option --bind-address
	if s.SecureServing.BindAddress.String() != "192.168.10.20" {
		t.Errorf("Expected s.SecureServing.BindAddress to be 192.168.10.20")
	}
	// Check option --client-ca-file
	if s.Authentication.ClientCert.ClientCA != "/client-ca" {
		t.Errorf("Expected s.Authentication.ClientCert.ClientCA to be /client-ca")
	}
	// Check option --cloud-config
	if s.CloudProvider.CloudConfigFile != "/cloud-config" {
		t.Errorf("Expected s.CloudProvider.CloudConfigFile to be /cloud-config")
	}
	// Check option --cloud-provider
	if s.CloudProvider.CloudProvider != "daocloud" {
		t.Errorf("Expected s.CloudProvider.CloudProvider to be daocloud")
	}
	// Check option --cors-allowed-origins
	if s.GenericServerRunOptions.CorsAllowedOriginList[1] != "10.10.10.200" {
		t.Errorf("Expected s.GenericServerRunOptions.CorsAllowedOriginList[1] to be 10.10.10.200")
	}
	// Check option --enable-aggregator-routing
	if !s.EnableAggregatorRouting {
		t.Errorf("Expected s.EnableAggregatorRouting to be true")
	}
	// Check option --enable-logs-handler
	if s.EnableLogsHandler {
		t.Errorf("Expected s.EnableLogsHandler to be false")
	}
	// Check option --enable-swagger-ui
	if !s.Features.EnableSwaggerUI {
		t.Errorf("Expected s.Features.EnableSwaggerUI to be true")
	}
}
