package awsutils

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// mockEC2API implements EC2API for testing
type mockEC2API struct {
	describeNetworkInterfacesFunc        func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error)
	deleteNetworkInterfaceFunc           func(ctx context.Context, params *ec2.DeleteNetworkInterfaceInput) (*ec2.DeleteNetworkInterfaceOutput, error)
	createNetworkInterfaceFunc           func(ctx context.Context, params *ec2.CreateNetworkInterfaceInput) (*ec2.CreateNetworkInterfaceOutput, error)
	terminateInstancesFunc               func(ctx context.Context, params *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error)
	runInstancesFunc                     func(ctx context.Context, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error)
	describeInstancesFunc                func(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	createTagsFunc                       func(ctx context.Context, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error)
	modifyInstanceAttributeFunc          func(ctx context.Context, input *ec2.ModifyInstanceAttributeInput) (*ec2.ModifyInstanceAttributeOutput, error)
	securityGroupNametoIDFunc            func(ctx context.Context, sgNames []string) ([]string, error)
	describeIamFunc                      func(ctx context.Context, input *ec2.DescribeIamInstanceProfileAssociationsInput) (*ec2.DescribeIamInstanceProfileAssociationsOutput, error)
	replaceIamFunc                       func(ctx context.Context, input *ec2.ReplaceIamInstanceProfileAssociationInput) (*ec2.ReplaceIamInstanceProfileAssociationOutput, error)
	newInstanceRunningWaiterFunc         func(input ec2.DescribeInstancesInput) error
}

func (m *mockEC2API) DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
	if m.describeNetworkInterfacesFunc != nil {
		return m.describeNetworkInterfacesFunc(ctx, params)
	}
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}

func (m *mockEC2API) DeleteNetworkInterface(ctx context.Context, params *ec2.DeleteNetworkInterfaceInput) (*ec2.DeleteNetworkInterfaceOutput, error) {
	if m.deleteNetworkInterfaceFunc != nil {
		return m.deleteNetworkInterfaceFunc(ctx, params)
	}
	return &ec2.DeleteNetworkInterfaceOutput{}, nil
}

func (m *mockEC2API) CreateNetworkInterface(ctx context.Context, params *ec2.CreateNetworkInterfaceInput) (*ec2.CreateNetworkInterfaceOutput, error) {
	if m.createNetworkInterfaceFunc != nil {
		return m.createNetworkInterfaceFunc(ctx, params)
	}
	return nil, errors.New("not implemented")
}

func (m *mockEC2API) TerminateInstances(ctx context.Context, params *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	if m.terminateInstancesFunc != nil {
		return m.terminateInstancesFunc(ctx, params)
	}
	return nil, errors.New("not implemented")
}

func (m *mockEC2API) RunInstances(ctx context.Context, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	if m.runInstancesFunc != nil {
		return m.runInstancesFunc(ctx, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockEC2API) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if m.describeInstancesFunc != nil {
		return m.describeInstancesFunc(ctx, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockEC2API) CreateTags(ctx context.Context, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	if m.createTagsFunc != nil {
		return m.createTagsFunc(ctx, input)
	}
	return &ec2.CreateTagsOutput{}, nil
}

func (m *mockEC2API) ModifyInstanceAttribute(ctx context.Context, input *ec2.ModifyInstanceAttributeInput) (*ec2.ModifyInstanceAttributeOutput, error) {
	if m.modifyInstanceAttributeFunc != nil {
		return m.modifyInstanceAttributeFunc(ctx, input)
	}
	return &ec2.ModifyInstanceAttributeOutput{}, nil
}

func (m *mockEC2API) SecurityGroupNametoID(ctx context.Context, sgNames []string) ([]string, error) {
	if m.securityGroupNametoIDFunc != nil {
		return m.securityGroupNametoIDFunc(ctx, sgNames)
	}
	return nil, errors.New("not implemented")
}

func (m *mockEC2API) DescribeIamInstanceProfileAssociations(ctx context.Context, input *ec2.DescribeIamInstanceProfileAssociationsInput) (*ec2.DescribeIamInstanceProfileAssociationsOutput, error) {
	if m.describeIamFunc != nil {
		return m.describeIamFunc(ctx, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockEC2API) ReplaceIamInstanceProfileAssociation(ctx context.Context, input *ec2.ReplaceIamInstanceProfileAssociationInput) (*ec2.ReplaceIamInstanceProfileAssociationOutput, error) {
	if m.replaceIamFunc != nil {
		return m.replaceIamFunc(ctx, input)
	}
	return nil, errors.New("not implemented")
}

func (m *mockEC2API) NewInstanceRunningWaiter(input ec2.DescribeInstancesInput) error {
	if m.newInstanceRunningWaiterFunc != nil {
		return m.newInstanceRunningWaiterFunc(input)
	}
	return nil
}

// --- Tests ---

func TestCreateNetworkInterface(t *testing.T) {
	mock := &mockEC2API{
		createNetworkInterfaceFunc: func(ctx context.Context, params *ec2.CreateNetworkInterfaceInput) (*ec2.CreateNetworkInterfaceOutput, error) {
			return &ec2.CreateNetworkInterfaceOutput{
				NetworkInterface: &types.NetworkInterface{
					PrivateDnsName:     aws.String("ip-10-0-0-1.ec2.internal"),
					NetworkInterfaceId: aws.String("eni-12345"),
				},
			}, nil
		},
	}

	ip, eniID, err := CreateNetworkInterface("test-tag", "subnet-123", mock)
	if err != nil {
		t.Fatalf("CreateNetworkInterface failed: %v", err)
	}
	if ip != "ip-10-0-0-1.ec2.internal" {
		t.Errorf("expected ip 'ip-10-0-0-1.ec2.internal', got %q", ip)
	}
	if eniID != "eni-12345" {
		t.Errorf("expected eniID 'eni-12345', got %q", eniID)
	}
}

func TestCreateNetworkInterfaceError(t *testing.T) {
	mock := &mockEC2API{
		createNetworkInterfaceFunc: func(ctx context.Context, params *ec2.CreateNetworkInterfaceInput) (*ec2.CreateNetworkInterfaceOutput, error) {
			return nil, errors.New("ec2 error")
		},
	}

	_, _, err := CreateNetworkInterface("test-tag", "subnet-123", mock)
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetNetworkInterfaceByTagName(t *testing.T) {
	mock := &mockEC2API{
		describeNetworkInterfacesFunc: func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []types.NetworkInterface{
					{
						PrivateDnsName:     aws.String("ip-10-0-0-2.ec2.internal"),
						NetworkInterfaceId: aws.String("eni-456"),
					},
				},
			}, nil
		},
	}

	dns, eniID, err := GetNetworkInterfaceByTagName("test-tag", mock)
	if err != nil {
		t.Fatalf("GetNetworkInterfaceByTagName failed: %v", err)
	}
	if dns != "ip-10-0-0-2.ec2.internal" {
		t.Errorf("expected dns 'ip-10-0-0-2.ec2.internal', got %q", dns)
	}
	if eniID != "eni-456" {
		t.Errorf("expected eniID 'eni-456', got %q", eniID)
	}
}

func TestGetNetworkInterfaceByTagNameNotFound(t *testing.T) {
	mock := &mockEC2API{
		describeNetworkInterfacesFunc: func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []types.NetworkInterface{},
			}, nil
		},
	}

	dns, eniID, err := GetNetworkInterfaceByTagName("nonexistent", mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dns != "" {
		t.Errorf("expected empty dns, got %q", dns)
	}
	if eniID != "" {
		t.Errorf("expected empty eniID, got %q", eniID)
	}
}

func TestDeleteNetworkInterface(t *testing.T) {
	mock := &mockEC2API{
		describeNetworkInterfacesFunc: func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []types.NetworkInterface{
					{
						PrivateDnsName:     aws.String("ip-10-0-0-1.ec2.internal"),
						NetworkInterfaceId: aws.String("eni-todelete"),
					},
				},
			}, nil
		},
		deleteNetworkInterfaceFunc: func(ctx context.Context, params *ec2.DeleteNetworkInterfaceInput) (*ec2.DeleteNetworkInterfaceOutput, error) {
			if *params.NetworkInterfaceId != "eni-todelete" {
				t.Errorf("expected eni-todelete, got %q", *params.NetworkInterfaceId)
			}
			return &ec2.DeleteNetworkInterfaceOutput{}, nil
		},
	}

	err := DeleteNetworkInterface("test-tag", mock)
	if err != nil {
		t.Fatalf("DeleteNetworkInterface failed: %v", err)
	}
}

func TestDeleteNetworkInterfaceNotFound(t *testing.T) {
	mock := &mockEC2API{
		describeNetworkInterfacesFunc: func(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput) (*ec2.DescribeNetworkInterfacesOutput, error) {
			return &ec2.DescribeNetworkInterfacesOutput{
				NetworkInterfaces: []types.NetworkInterface{},
			}, nil
		},
	}

	// Should return nil when ENI is not found
	err := DeleteNetworkInterface("nonexistent", mock)
	if err != nil {
		t.Fatalf("expected nil error for not-found ENI, got: %v", err)
	}
}

func TestGetInstanceStatusById(t *testing.T) {
	mock := &mockEC2API{
		describeInstancesFunc: func(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{
					{
						Instances: []types.Instance{
							{
								InstanceId:       aws.String("i-12345"),
								PrivateIpAddress: aws.String("10.0.0.5"),
								State: &types.InstanceState{
									Name: types.InstanceStateNameRunning,
								},
							},
						},
					},
				},
			}, nil
		},
	}

	status, ip, err := GetInstanceStatusById("i-12345", mock)
	if err != nil {
		t.Fatalf("GetInstanceStatusById failed: %v", err)
	}
	if status != "running" {
		t.Errorf("expected status 'running', got %q", status)
	}
	if ip != "10.0.0.5" {
		t.Errorf("expected ip '10.0.0.5', got %q", ip)
	}
}

func TestGetInstanceStatusByIdNotFound(t *testing.T) {
	mock := &mockEC2API{
		describeInstancesFunc: func(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{},
			}, nil
		},
	}

	_, _, err := GetInstanceStatusById("i-nonexistent", mock)
	if err == nil {
		t.Error("expected error for no reservations")
	}
}

func TestGetInstanceStatusByIdMultipleReservations(t *testing.T) {
	mock := &mockEC2API{
		describeInstancesFunc: func(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
			return &ec2.DescribeInstancesOutput{
				Reservations: []types.Reservation{{}, {}},
			}, nil
		},
	}

	_, _, err := GetInstanceStatusById("i-12345", mock)
	if err == nil {
		t.Error("expected error for multiple reservations")
	}
}

func TestGetInstanceStatusByIdAPIError(t *testing.T) {
	mock := &mockEC2API{
		describeInstancesFunc: func(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
			return nil, errors.New("api error")
		},
	}

	_, _, err := GetInstanceStatusById("i-12345", mock)
	if err == nil {
		t.Error("expected error from API")
	}
}

func TestUpdateInstanceSecurityGroupsWithIDs(t *testing.T) {
	modifyCalled := false
	mock := &mockEC2API{
		modifyInstanceAttributeFunc: func(ctx context.Context, input *ec2.ModifyInstanceAttributeInput) (*ec2.ModifyInstanceAttributeOutput, error) {
			modifyCalled = true
			if len(input.Groups) != 2 {
				t.Errorf("expected 2 security groups, got %d", len(input.Groups))
			}
			return &ec2.ModifyInstanceAttributeOutput{}, nil
		},
	}

	err := UpdateInstanceSecurityGroups(context.Background(), mock, "i-12345", []string{"sg-1", "sg-2"})
	if err != nil {
		t.Fatalf("UpdateInstanceSecurityGroups failed: %v", err)
	}
	if !modifyCalled {
		t.Error("expected ModifyInstanceAttribute to be called")
	}
}

func TestUpdateInstanceSecurityGroupsWithNames(t *testing.T) {
	mock := &mockEC2API{
		securityGroupNametoIDFunc: func(ctx context.Context, sgNames []string) ([]string, error) {
			return []string{"sg-resolved-1", "sg-resolved-2"}, nil
		},
		modifyInstanceAttributeFunc: func(ctx context.Context, input *ec2.ModifyInstanceAttributeInput) (*ec2.ModifyInstanceAttributeOutput, error) {
			if len(input.Groups) != 2 {
				t.Errorf("expected 2 groups, got %d", len(input.Groups))
			}
			if input.Groups[0] != "sg-resolved-1" {
				t.Errorf("expected 'sg-resolved-1', got %q", input.Groups[0])
			}
			return &ec2.ModifyInstanceAttributeOutput{}, nil
		},
	}

	err := UpdateInstanceSecurityGroups(context.Background(), mock, "i-12345", []string{"default", "web-tier"})
	if err != nil {
		t.Fatalf("UpdateInstanceSecurityGroups failed: %v", err)
	}
}

func TestUpdateInstanceSecurityGroupsEmpty(t *testing.T) {
	mock := &mockEC2API{}

	// Should return nil for empty SG list
	err := UpdateInstanceSecurityGroups(context.Background(), mock, "i-12345", []string{})
	if err != nil {
		t.Fatalf("expected nil for empty SG list, got: %v", err)
	}
}

func TestUpdateInstanceSecurityGroupsNameResolutionError(t *testing.T) {
	mock := &mockEC2API{
		securityGroupNametoIDFunc: func(ctx context.Context, sgNames []string) ([]string, error) {
			return nil, errors.New("cannot resolve SG names")
		},
	}

	// Name must NOT contain "sg-" substring to trigger name resolution path
	err := UpdateInstanceSecurityGroups(context.Background(), mock, "i-12345", []string{"my-group"})
	if err == nil {
		t.Error("expected error from SG name resolution")
	}
}
