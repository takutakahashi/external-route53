package healthcheck

import (
	"github.com/google/go-cmp/cmp"
	"testing"

	"github.com/google/uuid"
	route53v1 "github.com/takutakahashi/external-route53/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEnsureAndDelete(t *testing.T) {
	rv, _ := uuid.NewRandom()
	type args struct {
		h *route53v1.HealthCheck
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",

			args: args{
				h: &route53v1.HealthCheck{
					ObjectMeta: v1.ObjectMeta{
						Name:            "test",
						Namespace:       "test",
						ResourceVersion: rv.String(),
					},
					Spec: route53v1.HealthCheckSpec{
						Enabled:          true,
						Invert:           false,
						Protocol:         route53v1.ProtocolTCP,
						Port:             443,
						Path:             "/",
						FailureThreshold: 1,
						Endpoint: route53v1.HealthCheckEndpoint{
							Address: "8.8.8.8",
						},
						Features: route53v1.HealthCheckFeatures{
							FastInterval: true,
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := Ensure(tt.args.h)
			if (err != nil) != tt.wantErr {
				t.Errorf("Ensure() error = %v, wantErr %v", err, tt.wantErr)
			}
			if _, err := Delete(h); (err != nil) != tt.wantErr {
				t.Errorf("Ensure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildResource(t *testing.T) {
	type args struct {
		svc *corev1.Service
	}
	tests := []struct {
		name    string
		args    args
		want    *route53v1.HealthCheck
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Port: 30080},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "203.0.113.1"},
							},
						},
					},
				},
			},
			want: &route53v1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "test",
						},
					},
				},
				Spec: route53v1.HealthCheckSpec{
					Enabled:  true,
					Invert:   false,
					Protocol: route53v1.ProtocolTCP,
					Port:     int(30080),
					Endpoint: route53v1.HealthCheckEndpoint{
						Address: "203.0.113.1",
					},
					FailureThreshold: 3,
					Features: route53v1.HealthCheckFeatures{
						FastInterval: true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "hostname",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
						Ports: []corev1.ServicePort{
							{Port: 30080},
						},
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{Hostname: "test.elb.ap-northeast-1.amazonaws.com"},
							},
						},
					},
				},
			},
			want: &route53v1.HealthCheck{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
					OwnerReferences: []metav1.OwnerReference{
						{
							Name: "test",
						},
					},
				},
				Spec: route53v1.HealthCheckSpec{
					Enabled:  true,
					Invert:   false,
					Protocol: route53v1.ProtocolTCP,
					Port:     int(30080),
					Endpoint: route53v1.HealthCheckEndpoint{
						Hostname: "test.elb.ap-northeast-1.amazonaws.com",
					},
					FailureThreshold: 3,
					Features: route53v1.HealthCheckFeatures{
						FastInterval: true,
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildResource(tt.args.svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildResource() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(*got, *tt.want); diff != "" {
				t.Errorf("buildResource() value mismatch\n%s", diff)
			}
		})
	}
}
