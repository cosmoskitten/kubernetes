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

package aws

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const ProxyProtocolPolicyName = "k8s-proxyprotocol-enabled"

func isNLB(annotations map[string]string) bool {
	if annotations[ServiceAnnotationLoadBalancerType] == "nlb" {
		return true
	}
	return false
}

// getLoadBalancerAdditionalTags converts the comma separated list of key-value
// pairs in the ServiceAnnotationLoadBalancerAdditionalTags annotation and returns
// it as a map.
func getLoadBalancerAdditionalTags(annotations map[string]string) map[string]string {
	additionalTags := make(map[string]string)
	if additionalTagsList, ok := annotations[ServiceAnnotationLoadBalancerAdditionalTags]; ok {
		additionalTagsList = strings.TrimSpace(additionalTagsList)

		// Break up list of "Key1=Val,Key2=Val2"
		tagList := strings.Split(additionalTagsList, ",")

		// Break up "Key=Val"
		for _, tagSet := range tagList {
			tag := strings.Split(strings.TrimSpace(tagSet), "=")

			// Accept "Key=val" or "Key=" or just "Key"
			if len(tag) >= 2 && len(tag[0]) != 0 {
				// There is a key and a value, so save it
				additionalTags[tag[0]] = tag[1]
			} else if len(tag) == 1 && len(tag[0]) != 0 {
				// Just "Key"
				additionalTags[tag[0]] = ""
			}
		}
	}

	return additionalTags
}

// ensureLoadBalancerv2 ensures a v2 load balancer is created
//
// listeners is a map of frontend to NodePorts
func (c *Cloud) ensureLoadBalancerv2(namespacedName types.NamespacedName, loadBalancerName string, listeners map[int64]int64, instanceIDs, subnetIDs []string, internalELB bool, annotations map[string]string) (*elbv2.LoadBalancer, error) {
	loadBalancer, err := c.describeLoadBalancerv2(loadBalancerName)
	if err != nil {
		return nil, err
	}

	dirty := false

	// Get additional tags set by the user
	tags := getLoadBalancerAdditionalTags(annotations)
	// Add default tags
	tags[TagNameKubernetesService] = namespacedName.String()
	tags = c.tagging.buildTags(ResourceLifecycleOwned, tags)

	if loadBalancer == nil {
		// Create the LB
		createRequest := &elbv2.CreateLoadBalancerInput{}
		createRequest.Name = aws.String(loadBalancerName)
		if internalELB {
			createRequest.Scheme = aws.String("internal")
		}

		// We are supposed to specify one subnet per AZ.
		// TODO: What happens if we have more than one subnet per AZ?
		createRequest.SubnetMappings = createSubnetMappings(subnetIDs)

		for k, v := range tags {
			createRequest.Tags = append(createRequest.Tags, &elbv2.Tag{
				Key: aws.String(k), Value: aws.String(v),
			})
		}

		glog.Infof("Creating load balancer for %v with name: %s", namespacedName, loadBalancerName)
		createResponse, err := c.elbv2.CreateLoadBalancer(createRequest)
		if err != nil {
			return nil, err
		}

		// Create Target Groups
		addTagsInput := &elbv2.AddTagsInput{
			ResourceArns: []*string{},
			Tags:         []*elbv2.Tag{},
		}

		// It is way easier to keep track of updates by having possibly
		// duplicate target groups where the backend port is the same
		for frontendPort, nodePort := range listeners {
			_, targetGroupArn, err := c.createListenerV2(createResponse.LoadBalancers[0].LoadBalancerArn, frontendPort, nodePort, namespacedName, instanceIDs, *createResponse.LoadBalancers[0].VpcId)
			if err != nil {
				return nil, err
			}
			addTagsInput.ResourceArns = append(addTagsInput.ResourceArns, targetGroupArn)
		}

		// Add tags to targets
		for k, v := range tags {
			addTagsInput.Tags = append(addTagsInput.Tags, &elbv2.Tag{
				Key: aws.String(k), Value: aws.String(v),
			})
		}
		_, err = c.elbv2.AddTags(addTagsInput)
		if err != nil {
			return nil, err
		}
	} else {
		// TODO: Sync internal vs non-internal

		// sync listeners
		{
			// TODO handle paginated response here
			// TODO micahhausler
			listenerDescriptions, err := c.elbv2.DescribeListeners(
				&elbv2.DescribeListenersInput{
					LoadBalancerArn: loadBalancer.LoadBalancerArn,
				},
			)
			if err != nil {
				return nil, err
			}
			actual := map[int64]*elbv2.Listener{}
			for _, listener := range listenerDescriptions.Listeners {
				actual[*listener.Port] = listener
			}

			actualTargetGroups, err := c.elbv2.DescribeTargetGroups(
				&elbv2.DescribeTargetGroupsInput{
					LoadBalancerArn: loadBalancer.LoadBalancerArn,
				},
			)
			if err != nil {
				glog.Warning("Unable to create load balancer listener")
				return nil, err
			}
			nodePortTargetGroup := map[int64]*elbv2.TargetGroup{}
			for _, targetGroup := range actualTargetGroups.TargetGroups {
				nodePortTargetGroup[*targetGroup.Port] = targetGroup
			}

			// Create Target Groups
			addTagsInput := &elbv2.AddTagsInput{
				ResourceArns: []*string{},
				Tags:         []*elbv2.Tag{},
			}

			// Handle additions/modifications
			for frontendPort, nodePort := range listeners {

				// modifications
				if listener, ok := actual[frontendPort]; ok {

					// nodePort must have changed, we'll need to delete old TG
					// and recreate
					if targetGroup, ok := nodePortTargetGroup[nodePort]; !ok {
						// Create new Target group
						targetName := createTargetName(namespacedName, frontendPort, nodePort)
						targetGroup, err = c.ensureTargetGroup(
							nil,
							nodePort,
							instanceIDs,
							targetName,
							*loadBalancer.VpcId,
						)
						if err != nil {
							return nil, err
						}

						// Associate new target group to LB
						_, err := c.elbv2.ModifyListener(&elbv2.ModifyListenerInput{
							ListenerArn: listener.ListenerArn,
							Port:        aws.Int64(frontendPort),
							Protocol:    aws.String("TCP"),
							DefaultActions: []*elbv2.Action{{
								TargetGroupArn: targetGroup.TargetGroupArn,
								Type:           aws.String("forward"),
							}},
						})
						if err != nil {
							glog.Warning("Unable to update load balancer listener")
							return nil, err
						}

						// Delete old target group
						_, err = c.elbv2.DeleteTargetGroup(&elbv2.DeleteTargetGroupInput{
							TargetGroupArn: listener.DefaultActions[0].TargetGroupArn,
						})
						if err != nil {
							glog.Warning("Unable to delete old target group")
							return nil, err
						}

					} else {

						// Run ensureTargetGroup to make sure instances in service are up-to-date
						targetName := createTargetName(namespacedName, frontendPort, nodePort)
						_, err = c.ensureTargetGroup(
							targetGroup,
							nodePort,
							instanceIDs,
							targetName,
							*loadBalancer.VpcId,
						)
						if err != nil {
							return nil, err
						}
					}
					dirty = true
					continue
				}

				// Additions
				_, targetGroupArn, err := c.createListenerV2(loadBalancer.LoadBalancerArn, frontendPort, nodePort, namespacedName, instanceIDs, *loadBalancer.VpcId)
				if err != nil {
					return nil, err
				}
				addTagsInput.ResourceArns = append(addTagsInput.ResourceArns, targetGroupArn)
				dirty = true
			}

			// handle deletions
			for port, listener := range actual {
				if _, ok := listeners[port]; !ok {
					err := c.deleteListenerV2(listener)
					if err != nil {
						return nil, err
					}
					dirty = true
				}
			}

			// Add tags to new targets
			for k, v := range tags {
				addTagsInput.Tags = append(addTagsInput.Tags, &elbv2.Tag{
					Key: aws.String(k), Value: aws.String(v),
				})
			}
			_, err = c.elbv2.AddTags(addTagsInput)
			if err != nil {
				return nil, err
			}

		}

		// Subnets cannot be modified on NLBs

		if dirty {
			loadBalancers, err := c.elbv2.DescribeLoadBalancers(
				&elbv2.DescribeLoadBalancersInput{
					LoadBalancerArns: []*string{
						loadBalancer.LoadBalancerArn,
					},
				},
			)
			if err != nil {
				glog.Warning("Unable to retrieve load balancer after update")
				return nil, err
			}
			loadBalancer = loadBalancers.LoadBalancers[0]
		}
	}
	return loadBalancer, nil
}

func createTargetName(namespacedName types.NamespacedName, frontendPort, nodePort int64) string {
	return fmt.Sprintf("%s-%d-%d", namespacedName.String(), frontendPort, nodePort)
}

func (c *Cloud) createListenerV2(loadBalancerArn *string, frontendPort, nodePort int64, namespacedName types.NamespacedName, instanceIDs []string, vpcID string) (listener *elbv2.Listener, targetGroupArn *string, err error) {
	targetName := createTargetName(namespacedName, frontendPort, nodePort)

	glog.Infof("Creating load balancer target group for %v with name: %s", namespacedName, targetName)
	target, err := c.ensureTargetGroup(
		nil,
		nodePort,
		instanceIDs,
		targetName,
		vpcID,
	)
	if err != nil {
		return nil, aws.String(""), err
	}

	createListernerInput := &elbv2.CreateListenerInput{
		LoadBalancerArn: loadBalancerArn,
		Port:            aws.Int64(frontendPort),
		Protocol:        aws.String("TCP"),
		DefaultActions: []*elbv2.Action{{
			TargetGroupArn: target.TargetGroupArn,
			Type:           aws.String("forward"),
		}},
	}
	glog.Infof("Creating load balancer listener for %v", namespacedName)
	createListenerOutput, err := c.elbv2.CreateListener(createListernerInput)
	if err != nil {
		glog.Warning("Unable to create load balancer listener")
		return nil, aws.String(""), err
	}
	return createListenerOutput.Listeners[0], target.TargetGroupArn, nil
}

// cleans up listener and corresponding target group
func (c *Cloud) deleteListenerV2(listener *elbv2.Listener) error {
	_, err := c.elbv2.DeleteListener(&elbv2.DeleteListenerInput{ListenerArn: listener.ListenerArn})
	if err != nil {
		glog.Warning("Unable to delete load balancer listener")
		return err
	}
	_, err = c.elbv2.DeleteTargetGroup(&elbv2.DeleteTargetGroupInput{TargetGroupArn: listener.DefaultActions[0].TargetGroupArn})
	if err != nil {
		glog.Warning("Unable to delete load balancer target group")
	}
	return err
}

// ensureTargetGroup creates a target group with a set of instances
func (c *Cloud) ensureTargetGroup(targetGroup *elbv2.TargetGroup, nodePort int64, instances []string, name string, vpcID string) (*elbv2.TargetGroup, error) {
	dirty := false
	if targetGroup == nil {
		input := &elbv2.CreateTargetGroupInput{
			VpcId:                      aws.String(vpcID),
			Name:                       aws.String(name),
			Port:                       aws.Int64(nodePort),
			Protocol:                   aws.String("TCP"),
			TargetType:                 aws.String("instance"),
			HealthCheckIntervalSeconds: aws.Int64(30),
			HealthCheckPort:            aws.String("traffic-port"),
			HealthCheckProtocol:        aws.String("TCP"),
			HealthCheckTimeoutSeconds:  aws.Int64(10),
			HealthyThresholdCount:      aws.Int64(3),
			UnhealthyThresholdCount:    aws.Int64(3),
		}

		result, err := c.elbv2.CreateTargetGroup(input)
		if err != nil {
			glog.Warning("Unable to create load balancer target group")
			return nil, err
		}
		if len(result.TargetGroups) != 1 {
			return nil, fmt.Errorf("Expected only one target group on create!")
		}

		registerInput := &elbv2.RegisterTargetsInput{
			TargetGroupArn: result.TargetGroups[0].TargetGroupArn,
			Targets:        []*elbv2.TargetDescription{},
		}
		for _, instanceID := range instances {
			registerInput.Targets = append(registerInput.Targets, &elbv2.TargetDescription{
				Id:   aws.String(string(instanceID)),
				Port: aws.Int64(nodePort),
			})
		}

		_, err = c.elbv2.RegisterTargets(registerInput)
		if err != nil {
			glog.Warning("Unable register targets for load balancer")
			return nil, err
		}

		return result.TargetGroups[0], nil
	}

	// handle instances in service
	{
		healthResponse, err := c.elbv2.DescribeTargetHealth(&elbv2.DescribeTargetHealthInput{TargetGroupArn: targetGroup.TargetGroupArn})
		if err != nil {
			glog.Warning("Unable to describe target group health")
			return nil, err
		}
		responseIDs := []string{}
		for _, healthDescription := range healthResponse.TargetHealthDescriptions {
			responseIDs = append(responseIDs, *healthDescription.Target.Id)
		}

		actual := sets.NewString(instances...)
		expected := sets.NewString(responseIDs...)

		additions := expected.Difference(actual)
		removals := actual.Difference(expected)

		if len(additions) > 0 {
			registerInput := &elbv2.RegisterTargetsInput{
				TargetGroupArn: targetGroup.TargetGroupArn,
				Targets:        []*elbv2.TargetDescription{},
			}
			for instanceID := range additions {
				registerInput.Targets = append(registerInput.Targets, &elbv2.TargetDescription{
					Id:   aws.String(string(instanceID)),
					Port: aws.Int64(nodePort),
				})
			}
			_, err := c.elbv2.RegisterTargets(registerInput)
			if err != nil {
				glog.Warning("Unable to register new targets in target group")
				return nil, err
			}
			dirty = true
		}

		if len(removals) > 0 {
			deregisterInput := &elbv2.DeregisterTargetsInput{
				TargetGroupArn: targetGroup.TargetGroupArn,
				Targets:        []*elbv2.TargetDescription{},
			}
			for instanceID := range removals {
				deregisterInput.Targets = append(deregisterInput.Targets, &elbv2.TargetDescription{
					Id:   aws.String(string(instanceID)),
					Port: aws.Int64(nodePort),
				})
			}
			_, err := c.elbv2.DeregisterTargets(deregisterInput)
			if err != nil {
				glog.Warning("Unable to deregister targets in target group")
				return nil, err
			}
			dirty = true
		}
	}
	if dirty {
		result, err := c.elbv2.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
			Names: []*string{aws.String(name)},
		})
		if err != nil {
			glog.Warning("Unable to retrieve target group after creation/update")
			return nil, err
		}
		targetGroup = result.TargetGroups[0]
	}

	return targetGroup, nil
}

func (c *Cloud) ensureLoadBalancer(namespacedName types.NamespacedName, loadBalancerName string, listeners []*elb.Listener, subnetIDs []string, securityGroupIDs []string, internalELB, proxyProtocol bool, loadBalancerAttributes *elb.LoadBalancerAttributes, annotations map[string]string) (*elb.LoadBalancerDescription, error) {
	loadBalancer, err := c.describeLoadBalancer(loadBalancerName)
	if err != nil {
		return nil, err
	}

	dirty := false

	if loadBalancer == nil {
		createRequest := &elb.CreateLoadBalancerInput{}
		createRequest.LoadBalancerName = aws.String(loadBalancerName)

		createRequest.Listeners = listeners

		if internalELB {
			createRequest.Scheme = aws.String("internal")
		}

		// We are supposed to specify one subnet per AZ.
		// TODO: What happens if we have more than one subnet per AZ?
		createRequest.Subnets = stringPointerArray(subnetIDs)

		createRequest.SecurityGroups = stringPointerArray(securityGroupIDs)

		// Get additional tags set by the user
		tags := getLoadBalancerAdditionalTags(annotations)

		// Add default tags
		tags[TagNameKubernetesService] = namespacedName.String()
		tags = c.tagging.buildTags(ResourceLifecycleOwned, tags)

		for k, v := range tags {
			createRequest.Tags = append(createRequest.Tags, &elb.Tag{
				Key: aws.String(k), Value: aws.String(v),
			})
		}

		glog.Infof("Creating load balancer for %v with name: %s", namespacedName, loadBalancerName)
		_, err := c.elb.CreateLoadBalancer(createRequest)
		if err != nil {
			return nil, err
		}

		if proxyProtocol {
			err = c.createProxyProtocolPolicy(loadBalancerName)
			if err != nil {
				return nil, err
			}

			for _, listener := range listeners {
				glog.V(2).Infof("Adjusting AWS loadbalancer proxy protocol on node port %d. Setting to true", *listener.InstancePort)
				err := c.setBackendPolicies(loadBalancerName, *listener.InstancePort, []*string{aws.String(ProxyProtocolPolicyName)})
				if err != nil {
					return nil, err
				}
			}
		}

		dirty = true
	} else {
		// TODO: Sync internal vs non-internal

		{
			// Sync subnets
			expected := sets.NewString(subnetIDs...)
			actual := stringSetFromPointers(loadBalancer.Subnets)

			additions := expected.Difference(actual)
			removals := actual.Difference(expected)

			if removals.Len() != 0 {
				request := &elb.DetachLoadBalancerFromSubnetsInput{}
				request.LoadBalancerName = aws.String(loadBalancerName)
				request.Subnets = stringSetToPointers(removals)
				glog.V(2).Info("Detaching load balancer from removed subnets")
				_, err := c.elb.DetachLoadBalancerFromSubnets(request)
				if err != nil {
					return nil, fmt.Errorf("error detaching AWS loadbalancer from subnets: %q", err)
				}
				dirty = true
			}

			if additions.Len() != 0 {
				request := &elb.AttachLoadBalancerToSubnetsInput{}
				request.LoadBalancerName = aws.String(loadBalancerName)
				request.Subnets = stringSetToPointers(additions)
				glog.V(2).Info("Attaching load balancer to added subnets")
				_, err := c.elb.AttachLoadBalancerToSubnets(request)
				if err != nil {
					return nil, fmt.Errorf("error attaching AWS loadbalancer to subnets: %q", err)
				}
				dirty = true
			}
		}

		{
			// Sync security groups
			expected := sets.NewString(securityGroupIDs...)
			actual := stringSetFromPointers(loadBalancer.SecurityGroups)

			if !expected.Equal(actual) {
				// This call just replaces the security groups, unlike e.g. subnets (!)
				request := &elb.ApplySecurityGroupsToLoadBalancerInput{}
				request.LoadBalancerName = aws.String(loadBalancerName)
				request.SecurityGroups = stringPointerArray(securityGroupIDs)
				glog.V(2).Info("Applying updated security groups to load balancer")
				_, err := c.elb.ApplySecurityGroupsToLoadBalancer(request)
				if err != nil {
					return nil, fmt.Errorf("error applying AWS loadbalancer security groups: %q", err)
				}
				dirty = true
			}
		}

		{
			// Sync listeners
			listenerDescriptions := loadBalancer.ListenerDescriptions

			foundSet := make(map[int]bool)
			removals := []*int64{}
			for _, listenerDescription := range listenerDescriptions {
				actual := listenerDescription.Listener
				if actual == nil {
					glog.Warning("Ignoring empty listener in AWS loadbalancer: ", loadBalancerName)
					continue
				}

				found := -1
				for i, expected := range listeners {
					if elbProtocolsAreEqual(actual.Protocol, expected.Protocol) {
						continue
					}
					if elbProtocolsAreEqual(actual.InstanceProtocol, expected.InstanceProtocol) {
						continue
					}
					if orZero(actual.InstancePort) != orZero(expected.InstancePort) {
						continue
					}
					if orZero(actual.LoadBalancerPort) != orZero(expected.LoadBalancerPort) {
						continue
					}
					if awsArnEquals(actual.SSLCertificateId, expected.SSLCertificateId) {
						continue
					}
					found = i
				}
				if found != -1 {
					foundSet[found] = true
				} else {
					removals = append(removals, actual.LoadBalancerPort)
				}
			}

			additions := []*elb.Listener{}
			for i := range listeners {
				if foundSet[i] {
					continue
				}
				additions = append(additions, listeners[i])
			}

			if len(removals) != 0 {
				request := &elb.DeleteLoadBalancerListenersInput{}
				request.LoadBalancerName = aws.String(loadBalancerName)
				request.LoadBalancerPorts = removals
				glog.V(2).Info("Deleting removed load balancer listeners")
				_, err := c.elb.DeleteLoadBalancerListeners(request)
				if err != nil {
					return nil, fmt.Errorf("error deleting AWS loadbalancer listeners: %q", err)
				}
				dirty = true
			}

			if len(additions) != 0 {
				request := &elb.CreateLoadBalancerListenersInput{}
				request.LoadBalancerName = aws.String(loadBalancerName)
				request.Listeners = additions
				glog.V(2).Info("Creating added load balancer listeners")
				_, err := c.elb.CreateLoadBalancerListeners(request)
				if err != nil {
					return nil, fmt.Errorf("error creating AWS loadbalancer listeners: %q", err)
				}
				dirty = true
			}
		}

		{
			// Sync proxy protocol state for new and existing listeners

			proxyPolicies := make([]*string, 0)
			if proxyProtocol {
				// Ensure the backend policy exists

				// NOTE The documentation for the AWS API indicates we could get an HTTP 400
				// back if a policy of the same name already exists. However, the aws-sdk does not
				// seem to return an error to us in these cases. Therefore, this will issue an API
				// request every time.
				err := c.createProxyProtocolPolicy(loadBalancerName)
				if err != nil {
					return nil, err
				}

				proxyPolicies = append(proxyPolicies, aws.String(ProxyProtocolPolicyName))
			}

			foundBackends := make(map[int64]bool)
			proxyProtocolBackends := make(map[int64]bool)
			for _, backendListener := range loadBalancer.BackendServerDescriptions {
				foundBackends[*backendListener.InstancePort] = false
				proxyProtocolBackends[*backendListener.InstancePort] = proxyProtocolEnabled(backendListener)
			}

			for _, listener := range listeners {
				setPolicy := false
				instancePort := *listener.InstancePort

				if currentState, ok := proxyProtocolBackends[instancePort]; !ok {
					// This is a new ELB backend so we only need to worry about
					// potentially adding a policy and not removing an
					// existing one
					setPolicy = proxyProtocol
				} else {
					foundBackends[instancePort] = true
					// This is an existing ELB backend so we need to determine
					// if the state changed
					setPolicy = (currentState != proxyProtocol)
				}

				if setPolicy {
					glog.V(2).Infof("Adjusting AWS loadbalancer proxy protocol on node port %d. Setting to %t", instancePort, proxyProtocol)
					err := c.setBackendPolicies(loadBalancerName, instancePort, proxyPolicies)
					if err != nil {
						return nil, err
					}
					dirty = true
				}
			}

			// We now need to figure out if any backend policies need removed
			// because these old policies will stick around even if there is no
			// corresponding listener anymore
			for instancePort, found := range foundBackends {
				if !found {
					glog.V(2).Infof("Adjusting AWS loadbalancer proxy protocol on node port %d. Setting to false", instancePort)
					err := c.setBackendPolicies(loadBalancerName, instancePort, []*string{})
					if err != nil {
						return nil, err
					}
					dirty = true
				}
			}
		}
	}

	// Whether the ELB was new or existing, sync attributes regardless. This accounts for things
	// that cannot be specified at the time of creation and can only be modified after the fact,
	// e.g. idle connection timeout.
	{
		describeAttributesRequest := &elb.DescribeLoadBalancerAttributesInput{}
		describeAttributesRequest.LoadBalancerName = aws.String(loadBalancerName)
		describeAttributesOutput, err := c.elb.DescribeLoadBalancerAttributes(describeAttributesRequest)
		if err != nil {
			glog.Warning("Unable to retrieve load balancer attributes during attribute sync")
			return nil, err
		}

		foundAttributes := &describeAttributesOutput.LoadBalancerAttributes

		// Update attributes if they're dirty
		if !reflect.DeepEqual(loadBalancerAttributes, foundAttributes) {
			glog.V(2).Infof("Updating load-balancer attributes for %q", loadBalancerName)

			modifyAttributesRequest := &elb.ModifyLoadBalancerAttributesInput{}
			modifyAttributesRequest.LoadBalancerName = aws.String(loadBalancerName)
			modifyAttributesRequest.LoadBalancerAttributes = loadBalancerAttributes
			_, err = c.elb.ModifyLoadBalancerAttributes(modifyAttributesRequest)
			if err != nil {
				return nil, fmt.Errorf("Unable to update load balancer attributes during attribute sync: %q", err)
			}
			dirty = true
		}
	}

	if dirty {
		loadBalancer, err = c.describeLoadBalancer(loadBalancerName)
		if err != nil {
			glog.Warning("Unable to retrieve load balancer after creation/update")
			return nil, err
		}
	}

	return loadBalancer, nil
}

func createSubnetMappings(subnetIDs []string) []*elbv2.SubnetMapping {
	response := []*elbv2.SubnetMapping{}

	for _, id := range subnetIDs {
		// Ignore AllocationId for now
		response = append(response, &elbv2.SubnetMapping{SubnetId: aws.String(id)})
	}

	return response
}

// elbProtocolsAreEqual checks if two ELB protocol strings are considered the same
// Comparison is case insensitive
func elbProtocolsAreEqual(l, r *string) bool {
	if l == nil || r == nil {
		return l == r
	}
	return strings.EqualFold(aws.StringValue(l), aws.StringValue(r))
}

// awsArnEquals checks if two ARN strings are considered the same
// Comparison is case insensitive
func awsArnEquals(l, r *string) bool {
	if l == nil || r == nil {
		return l == r
	}
	return strings.EqualFold(aws.StringValue(l), aws.StringValue(r))
}

// Makes sure that the health check for an ELB matches the configured health check node port
func (c *Cloud) ensureLoadBalancerHealthCheck(loadBalancer *elb.LoadBalancerDescription, protocol string, port int32, path string) error {
	name := aws.StringValue(loadBalancer.LoadBalancerName)

	actual := loadBalancer.HealthCheck

	// Default AWS settings
	expectedHealthyThreshold := int64(2)
	expectedUnhealthyThreshold := int64(6)
	expectedTimeout := int64(5)
	expectedInterval := int64(10)

	expectedTarget := protocol + ":" + strconv.FormatInt(int64(port), 10) + path

	if expectedTarget == aws.StringValue(actual.Target) &&
		expectedHealthyThreshold == orZero(actual.HealthyThreshold) &&
		expectedUnhealthyThreshold == orZero(actual.UnhealthyThreshold) &&
		expectedTimeout == orZero(actual.Timeout) &&
		expectedInterval == orZero(actual.Interval) {
		return nil
	}

	glog.V(2).Infof("Updating load-balancer health-check for %q", name)

	healthCheck := &elb.HealthCheck{}
	healthCheck.HealthyThreshold = &expectedHealthyThreshold
	healthCheck.UnhealthyThreshold = &expectedUnhealthyThreshold
	healthCheck.Timeout = &expectedTimeout
	healthCheck.Interval = &expectedInterval
	healthCheck.Target = &expectedTarget

	request := &elb.ConfigureHealthCheckInput{}
	request.HealthCheck = healthCheck
	request.LoadBalancerName = loadBalancer.LoadBalancerName

	_, err := c.elb.ConfigureHealthCheck(request)
	if err != nil {
		return fmt.Errorf("error configuring load-balancer health-check for %q: %q", name, err)
	}

	return nil
}

// Makes sure that exactly the specified hosts are registered as instances with the load balancer
func (c *Cloud) ensureLoadBalancerInstances(loadBalancerName string, lbInstances []*elb.Instance, instanceIDs map[awsInstanceID]*ec2.Instance) error {
	expected := sets.NewString()
	for id := range instanceIDs {
		expected.Insert(string(id))
	}

	actual := sets.NewString()
	for _, lbInstance := range lbInstances {
		actual.Insert(aws.StringValue(lbInstance.InstanceId))
	}

	additions := expected.Difference(actual)
	removals := actual.Difference(expected)

	addInstances := []*elb.Instance{}
	for _, instanceId := range additions.List() {
		addInstance := &elb.Instance{}
		addInstance.InstanceId = aws.String(instanceId)
		addInstances = append(addInstances, addInstance)
	}

	removeInstances := []*elb.Instance{}
	for _, instanceId := range removals.List() {
		removeInstance := &elb.Instance{}
		removeInstance.InstanceId = aws.String(instanceId)
		removeInstances = append(removeInstances, removeInstance)
	}

	if len(addInstances) > 0 {
		registerRequest := &elb.RegisterInstancesWithLoadBalancerInput{}
		registerRequest.Instances = addInstances
		registerRequest.LoadBalancerName = aws.String(loadBalancerName)
		_, err := c.elb.RegisterInstancesWithLoadBalancer(registerRequest)
		if err != nil {
			return err
		}
		glog.V(1).Infof("Instances added to load-balancer %s", loadBalancerName)
	}

	if len(removeInstances) > 0 {
		deregisterRequest := &elb.DeregisterInstancesFromLoadBalancerInput{}
		deregisterRequest.Instances = removeInstances
		deregisterRequest.LoadBalancerName = aws.String(loadBalancerName)
		_, err := c.elb.DeregisterInstancesFromLoadBalancer(deregisterRequest)
		if err != nil {
			return err
		}
		glog.V(1).Infof("Instances removed from load-balancer %s", loadBalancerName)
	}

	return nil
}

func (c *Cloud) createProxyProtocolPolicy(loadBalancerName string) error {
	request := &elb.CreateLoadBalancerPolicyInput{
		LoadBalancerName: aws.String(loadBalancerName),
		PolicyName:       aws.String(ProxyProtocolPolicyName),
		PolicyTypeName:   aws.String("ProxyProtocolPolicyType"),
		PolicyAttributes: []*elb.PolicyAttribute{
			{
				AttributeName:  aws.String("ProxyProtocol"),
				AttributeValue: aws.String("true"),
			},
		},
	}
	glog.V(2).Info("Creating proxy protocol policy on load balancer")
	_, err := c.elb.CreateLoadBalancerPolicy(request)
	if err != nil {
		return fmt.Errorf("error creating proxy protocol policy on load balancer: %q", err)
	}

	return nil
}

func (c *Cloud) setBackendPolicies(loadBalancerName string, instancePort int64, policies []*string) error {
	request := &elb.SetLoadBalancerPoliciesForBackendServerInput{
		InstancePort:     aws.Int64(instancePort),
		LoadBalancerName: aws.String(loadBalancerName),
		PolicyNames:      policies,
	}
	if len(policies) > 0 {
		glog.V(2).Infof("Adding AWS loadbalancer backend policies on node port %d", instancePort)
	} else {
		glog.V(2).Infof("Removing AWS loadbalancer backend policies on node port %d", instancePort)
	}
	_, err := c.elb.SetLoadBalancerPoliciesForBackendServer(request)
	if err != nil {
		return fmt.Errorf("error adjusting AWS loadbalancer backend policies: %q", err)
	}

	return nil
}

func proxyProtocolEnabled(backend *elb.BackendServerDescription) bool {
	for _, policy := range backend.PolicyNames {
		if aws.StringValue(policy) == ProxyProtocolPolicyName {
			return true
		}
	}

	return false
}

// findInstancesForELB gets the EC2 instances corresponding to the Nodes, for setting up an ELB
// We ignore Nodes (with a log message) where the instanceid cannot be determined from the provider,
// and we ignore instances which are not found
func (c *Cloud) findInstancesForELB(nodes []*v1.Node) (map[awsInstanceID]*ec2.Instance, error) {
	// Map to instance ids ignoring Nodes where we cannot find the id (but logging)
	instanceIDs := mapToAWSInstanceIDsTolerant(nodes)

	cacheCriteria := cacheCriteria{
		// MaxAge not required, because we only care about security groups, which should not change
		HasInstances: instanceIDs, // Refresh if any of the instance ids are missing
	}
	snapshot, err := c.instanceCache.describeAllInstancesCached(cacheCriteria)
	if err != nil {
		return nil, err
	}

	instances := snapshot.FindInstances(instanceIDs)
	// We ignore instances that cannot be found

	return instances, nil
}
