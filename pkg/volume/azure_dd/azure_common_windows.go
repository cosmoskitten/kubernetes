// +build windows

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

package azure_dd

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"k8s.io/utils/exec"
)

func scsiHostRescan(io ioHandler) {
	//empty implementation here, don't need to rescan SCSI in Windows
}

func findDiskByLun(lun int, iohandler ioHandler, exe exec.Interface) (string, error) {
	cmd := `Get-Disk | Where-Object { $_.location.contains("LUN") } | select number, location`
	ex := exec.New()
	output, err := ex.Command("powershell", "/c", cmd).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Get-Disk failed in findDiskByLun, error: %v, output: %q", err, string(output))
	}

	if len(string(output)) < 10 {
		return "", fmt.Errorf("Get-Disk output is too short, output: %q", string(output))
	}

	reader := bufio.NewReader(strings.NewReader(string(output)))
	for {
		line, readerr := reader.ReadString('\n')

		arr := strings.Split(line, " LUN ")
		if len(arr) >= 2 {
			trimStr := strings.TrimRight(arr[1], "\r\n")
			trimStr = strings.TrimSpace(trimStr)
			l, err := strconv.Atoi(trimStr)
			if err == nil {
				if l == lun {
					trimStr = strings.TrimSpace(line)
					arr = strings.Split(trimStr, " ")
					if len(arr) >= 1 {
						n, err := strconv.Atoi(arr[0])
						if err == nil {
							glog.V(4).Infof("windowsDisk Mount: got disk number(%d) by LUN(%d)", n, lun)
							return strconv.Itoa(n), nil
						}
					}
					return "", fmt.Errorf("LUN(%d) found, but could not get disk number", lun)
				}
			}
		}

		if readerr != nil {
			if readerr != io.EOF {
				glog.Errorf("Unexpected error in findDiskByLun: %v", readerr)
			}
			break
		}
	}

	return "", nil
}

func formatIfNotFormatted(disk string, fstype string) {
	if err := validateDiskNumber(disk); err != nil {
		glog.Errorf("windowsDisk Mount: formatIfNotFormatted failed, err: %v\n", err)
		return
	}

	cmd := fmt.Sprintf("Get-Disk -Number %s | Where partitionstyle -eq 'raw' | Initialize-Disk -PartitionStyle MBR -PassThru", disk)
	cmd += " | New-Partition -AssignDriveLetter -UseMaximumSize | Format-Volume -FileSystem NTFS -Confirm:$false"
	ex := exec.New()
	output, err := ex.Command("powershell", "/c", cmd).CombinedOutput()
	if err != nil {
		glog.Errorf("windowsDisk Mount: Get-Disk failed, error: %v, output: %q", err, string(output))
	} else {
		glog.Infof("windowsDisk Mount: Disk successfully formatted, disk: %q, fstype: %q\n", disk, fstype)
	}
}

// disk number should be a number in [0, 99]
func validateDiskNumber(disk string) error {
	if len(disk) < 1 || len(disk) > 2 {
		return fmt.Errorf("wrong disk number format: %q", disk)
	}

	_, err := strconv.Atoi(disk)
	return err
}
