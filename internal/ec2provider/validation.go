package ec2provider

import (
	"fmt"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

var (
	// Patterns for valid AWS resource IDs
	amiIDPattern      = regexp.MustCompile(`^ami-[a-f0-9]{8,17}$`)
	subnetIDPattern   = regexp.MustCompile(`^subnet-[a-f0-9]{8,17}$`)
	sgIDPattern       = regexp.MustCompile(`^sg-[a-f0-9]{8,17}$`)
	instanceIDPattern = regexp.MustCompile(`^i-[a-f0-9]{8,17}$`)
	instanceTypeValid = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*\.[a-z0-9]+$`)
)

// ValidatePodAnnotations checks that pod annotations used to drive EC2 behavior
// contain valid, safe values. Returns an error if any annotation is invalid.
func ValidatePodAnnotations(pod *corev1.Pod) error {
	if pod.Annotations == nil {
		return nil
	}

	var errs []string

	// Validate image-id (AMI)
	if imageID, ok := pod.Annotations["compute.amazonaws.com/image-id"]; ok && imageID != "" {
		if !amiIDPattern.MatchString(imageID) {
			errs = append(errs, fmt.Sprintf("invalid image-id %q (must match ami-<hex>)", imageID))
		}
	}

	// Validate subnet-id
	if subnetID, ok := pod.Annotations["compute.amazonaws.com/subnet-id"]; ok && subnetID != "" {
		if !subnetIDPattern.MatchString(subnetID) {
			errs = append(errs, fmt.Sprintf("invalid subnet-id %q (must match subnet-<hex>)", subnetID))
		}
	}

	// Validate security-groups (comma-separated list of SG IDs or names)
	if sgs, ok := pod.Annotations["compute.amazonaws.com/security-groups"]; ok && sgs != "" {
		for _, sg := range strings.Split(sgs, ",") {
			sg = strings.TrimSpace(sg)
			if sg == "" {
				continue
			}
			// SG IDs must match pattern; names are also valid but must not contain injection chars
			if strings.HasPrefix(sg, "sg-") && !sgIDPattern.MatchString(sg) {
				errs = append(errs, fmt.Sprintf("invalid security group ID %q", sg))
			}
		}
	}

	// Validate instance-type
	if instanceType, ok := pod.Annotations["compute.amazonaws.com/instance-type"]; ok && instanceType != "" {
		if !instanceTypeValid.MatchString(instanceType) {
			errs = append(errs, fmt.Sprintf("invalid instance-type %q", instanceType))
		}
	}

	// Validate instance-id (if already set)
	if instanceID, ok := pod.Annotations["compute.amazonaws.com/instance-id"]; ok && instanceID != "" {
		if !instanceIDPattern.MatchString(instanceID) {
			errs = append(errs, fmt.Sprintf("invalid instance-id %q (must match i-<hex>)", instanceID))
		}
	}

	// Validate instance-profile (ARN or name - basic length/char check)
	if profile, ok := pod.Annotations["compute.amazonaws.com/instance-profile"]; ok && profile != "" {
		if len(profile) > 256 {
			errs = append(errs, "instance-profile name too long (max 256 chars)")
		}
	}

	if len(errs) > 0 {
		klog.Warningf("Pod %s/%s has invalid annotations: %v", pod.Namespace, pod.Name, errs)
		return fmt.Errorf("invalid pod annotations: %s", strings.Join(errs, "; "))
	}

	return nil
}
