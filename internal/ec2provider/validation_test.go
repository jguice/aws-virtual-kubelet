package ec2provider

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

func podWithAnnotations(annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod",
			Namespace:   "default",
			Annotations: annotations,
		},
	}
}

func TestValidatePodAnnotationsValid(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		"compute.amazonaws.com/image-id":      "ami-0123456789abcdef0",
		"compute.amazonaws.com/subnet-id":     "subnet-abcdef12",
		"compute.amazonaws.com/instance-type":  "m5.xlarge",
		"compute.amazonaws.com/instance-id":    "i-0123456789abcdef0",
		"compute.amazonaws.com/security-groups": "sg-12345678, sg-abcdef01",
		"compute.amazonaws.com/instance-profile": "my-instance-profile",
	})

	err := ValidatePodAnnotations(pod)
	if err != nil {
		t.Fatalf("expected valid annotations, got error: %v", err)
	}
}

func TestValidatePodAnnotationsNilAnnotations(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}
	err := ValidatePodAnnotations(pod)
	if err != nil {
		t.Fatalf("expected nil error for nil annotations, got: %v", err)
	}
}

func TestValidatePodAnnotationsEmptyAnnotations(t *testing.T) {
	pod := podWithAnnotations(map[string]string{})
	err := ValidatePodAnnotations(pod)
	if err != nil {
		t.Fatalf("expected nil error for empty annotations, got: %v", err)
	}
}

func TestValidatePodAnnotationsInvalidAMI(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"no prefix", "0123456789abcdef0"},
		{"wrong prefix", "emi-0123456789abcdef0"},
		{"too short", "ami-123"},
		{"injection", "ami-0123456789abcdef0; rm -rf /"},
		{"spaces", "ami-01234 56789"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := podWithAnnotations(map[string]string{
				"compute.amazonaws.com/image-id": tt.value,
			})
			err := ValidatePodAnnotations(pod)
			if err == nil {
				t.Errorf("expected error for invalid AMI %q", tt.value)
			}
		})
	}
}

func TestValidatePodAnnotationsInvalidSubnet(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		"compute.amazonaws.com/subnet-id": "not-a-subnet",
	})
	err := ValidatePodAnnotations(pod)
	if err == nil {
		t.Error("expected error for invalid subnet-id")
	}
}

func TestValidatePodAnnotationsInvalidInstanceType(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty dots", ".xlarge"},
		{"uppercase", "M5.xlarge"},
		{"special chars", "m5.x;large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := podWithAnnotations(map[string]string{
				"compute.amazonaws.com/instance-type": tt.value,
			})
			err := ValidatePodAnnotations(pod)
			if err == nil {
				t.Errorf("expected error for invalid instance-type %q", tt.value)
			}
		})
	}
}

func TestValidatePodAnnotationsValidInstanceTypes(t *testing.T) {
	validTypes := []string{
		"t3.micro", "m5.xlarge", "c6g.2xlarge", "mac2.metal",
		"mac2-m2.metal", "p4d.24xlarge", "r6i.large",
	}

	for _, it := range validTypes {
		t.Run(it, func(t *testing.T) {
			pod := podWithAnnotations(map[string]string{
				"compute.amazonaws.com/instance-type": it,
			})
			err := ValidatePodAnnotations(pod)
			if err != nil {
				t.Errorf("expected valid instance-type %q, got error: %v", it, err)
			}
		})
	}
}

func TestValidatePodAnnotationsInvalidInstanceID(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		"compute.amazonaws.com/instance-id": "notanid",
	})
	err := ValidatePodAnnotations(pod)
	if err == nil {
		t.Error("expected error for invalid instance-id")
	}
}

func TestValidatePodAnnotationsInvalidSGID(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		"compute.amazonaws.com/security-groups": "sg-tooshort",
	})
	err := ValidatePodAnnotations(pod)
	if err == nil {
		t.Error("expected error for invalid security group ID")
	}
}

func TestValidatePodAnnotationsProfileTooLong(t *testing.T) {
	longName := ""
	for i := 0; i < 260; i++ {
		longName += "a"
	}
	pod := podWithAnnotations(map[string]string{
		"compute.amazonaws.com/instance-profile": longName,
	})
	err := ValidatePodAnnotations(pod)
	if err == nil {
		t.Error("expected error for overly long instance-profile")
	}
}

func TestValidatePodAnnotationsMultipleErrors(t *testing.T) {
	pod := podWithAnnotations(map[string]string{
		"compute.amazonaws.com/image-id":   "bad-ami",
		"compute.amazonaws.com/subnet-id":  "bad-subnet",
		"compute.amazonaws.com/instance-id": "bad-instance",
	})
	err := ValidatePodAnnotations(pod)
	if err == nil {
		t.Fatal("expected error for multiple invalid annotations")
	}
}
