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

package e2e_node

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/blang/semver"
	"github.com/docker/docker/client"
)

const (
	defaultDockerEndpoint  = "unix:///var/run/docker.sock"
	dockerDaemonConfigName = "/etc/docker/daemon.json"
)

// getDockerAPIVersion returns the Docker's API version.
func getDockerAPIVersion() (semver.Version, error) {
	c, err := client.NewClient(defaultDockerEndpoint, "", nil, nil)
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to create docker client: %v", err)
	}
	version, err := c.ServerVersion(context.Background())
	if err != nil {
		return semver.Version{}, fmt.Errorf("failed to get docker server version: %v", err)
	}
	return semver.MustParse(version.APIVersion + ".0"), nil
}

// isSharedPIDNamespaceEnabled returns true if the Docker version is 1.13.1+
// (API version 1.26+), and false otherwise.
func isSharedPIDNamespaceEnabled() (bool, error) {
	version, err := getDockerAPIVersion()
	if err != nil {
		return false, err
	}
	return version.GTE(semver.MustParse("1.26.0")), nil
}

// isDockerNoNewPrivilegesSupported returns true if Docker version is 1.11+
// (API version 1.23+), and false otherwise.
func isDockerNoNewPrivilegesSupported() (bool, error) {
	version, err := getDockerAPIVersion()
	if err != nil {
		return false, err
	}
	return version.GTE(semver.MustParse("1.23.0")), nil
}

// isDockerLiveRestoreSupported returns true if live-restore is supported in
// the current Docker version.
func isDockerLiveRestoreSupported() (bool, error) {
	version, err := getDockerAPIVersion()
	if err != nil {
		return false, err
	}
	return version.GTE(semver.MustParse("1.26.0")), nil
}

// isDockerLiveRestoreEnabled returns true if live-restore is enabled in the
// Docker.
func isDockerLiveRestoreEnabled() (bool, error) {
	c, err := client.NewClient(defaultDockerEndpoint, "", nil, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create docker client: %v", err)
	}
	info, err := c.Info(context.Background())
	if err != nil {
		return false, fmt.Errorf("failed to get docker info: %v", err)
	}
	return info.LiveRestoreEnabled, nil
}

// setDockerLiveRestore sets the live-restore config to the specified value and
// then restarts the Docker daemon.
func setDockerLiveRestore(value bool) error {
	config, err := getDockerDaemonConfig()
	if err != nil {
		return err
	}
	config["live-restore"] = value
	if err := setDockerDaemonConfig(config); err != nil {
		return err
	}
	return restartDockerDaemon()
}

// stopDockerDaemon starts the Docker daemon.
func startDockerDaemon() error {
	switch {
	case isSystemd():
		_, err := runCommand("systemctl", "start", "docker")
		return err
	default:
		_, err := runCommand("service", "docker", "start")
		return err
	}
}

// stopDockerDaemon stops the Docker daemon.
func stopDockerDaemon() error {
	switch {
	case isSystemd():
		_, err := runCommand("systemctl", "stop", "docker")
		return err
	default:
		_, err := runCommand("service", "docker", "stop")
		return err
	}
}

// restartDockerDaemon restarts the Docker daemon.
func restartDockerDaemon() error {
	switch {
	case isSystemd():
		_, err := runCommand("systemctl", "restart", "docker")
		return err
	default:
		_, err := runCommand("service", "docker", "restart")
		return err
	}
}

// getDockerDaemonConfig returns the deserialized Docker daemon config from the
// 'dockerDaemonConfigName'.
func getDockerDaemonConfig() (map[string]interface{}, error) {
	if _, err := os.Stat(dockerDaemonConfigName); os.IsNotExist(err) {
		return make(map[string]interface{}), nil
	}
	data, err := ioutil.ReadFile(dockerDaemonConfigName)
	if err != nil {
		return nil, err
	}
	var config interface{}
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return config.(map[string]interface{}), nil
}

// setDockerDaemonConfig writes the serialized Docker daemon config to the
// configuration file 'dockerDaemonConfigName'.
func setDockerDaemonConfig(config map[string]interface{}) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(dockerDaemonConfigName, data, 0644); err != nil {
		return err
	}
	return nil
}
