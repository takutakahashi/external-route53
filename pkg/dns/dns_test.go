package dns

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var ROs []UpsertRecordSetOpt = []UpsertRecordSetOpt{
	{
		Hostname:        "external-route53.test.takutakahashi.dev.",
		Type:            "A",
		Identifier:      "/api/v1/namespaces/shared/services/test",
		HealthCheckID:   "",
		HostedZoneID:    "Z09261522C0IVI11TUTK7",
		Weight:          10,
		TTL:             300,
		Alias:           false,
		TargetIPAddress: "10.10.0.1",
	},
	{
		Hostname:        "external-route53.test.takutakahashi.dev.",
		Type:            "A",
		Identifier:      "/api/v1/namespaces/beta/services/test",
		HealthCheckID:   "",
		HostedZoneID:    "Z09261522C0IVI11TUTK7",
		Weight:          1,
		TTL:             300,
		Alias:           false,
		TargetIPAddress: "10.10.1.1",
	},
	{
		Hostname:       "not.test.takutakahashi.dev.",
		Type:           "A",
		Identifier:     "/api/v1/namespaces/beta/services/test",
		HealthCheckID:  "",
		HostedZoneID:   "Z09261522C0IVI11TUTK7",
		Weight:         1,
		TTL:            30,
		Alias:          true,
		TargetHostname: "external-route53.test.takutakahashi.dev.",
	},
}

func Test_ensureRecord(t *testing.T) {
	type args struct {
		ro UpsertRecordSetOpt
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{ro: ROs[0]},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ensureRecord(tt.args.ro); (err != nil) != tt.wantErr {
				t.Errorf("ensureRecord() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_recordExists(t *testing.T) {
	type args struct {
		ro UpsertRecordSetOpt
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name:    "ok",
			args:    args{ro: ROs[0]},
			want:    true,
			wantErr: false,
		},
		{
			name:    "ok",
			args:    args{ro: ROs[1]},
			want:    true,
			wantErr: false,
		},
		{
			name:    "ng",
			args:    args{ro: ROs[2]},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := recordExists(tt.args.ro)
			if (err != nil) != tt.wantErr {
				t.Errorf("recordExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("recordExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_upsert(t *testing.T) {
	type args struct {
		ro UpsertRecordSetOpt
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				ro: ROs[0],
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := upsert(tt.args.ro); (err != nil) != tt.wantErr {
				t.Errorf("upsert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_delete(t *testing.T) {
	type args struct {
		ro UpsertRecordSetOpt
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				ro: ROs[0],
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := delete(tt.args.ro); (err != nil) != tt.wantErr {
				t.Errorf("delete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_toUpsertRecordSetOpt(t *testing.T) {
	type args struct {
		svc *corev1.Service
	}
	tests := []struct {
		name    string
		args    args
		want    UpsertRecordSetOpt
		wantErr bool
	}{
		{
			name: "loadbalancer",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Annotations: map[string]string{
							HostnameAnnotationKey:      "test.test.example.com",
							aliasAnnotationKey:         "false",
							ttlAnnotationKey:           "10",
							healhCheckIdAnnotationKey:  "",
							weightAnnotationKey:        "1",
							setIdentifierAnnotationKey: "test/test",
							recordTypeAnnotationKey:    "A",
							HealthCheckAnnotationKey:   "enable",
							zoneAnnotationKey:          "test",
						},
						UID: "aaa",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "10.10.10.1"},
							},
						},
					},
				},
			},
			want: UpsertRecordSetOpt{
				Hostname:        "test.test.example.com",
				Type:            "A",
				Identifier:      "test/test",
				HealthCheckID:   "",
				HostedZoneID:    "test",
				Weight:          1,
				TTL:             10,
				Alias:           false,
				TargetHostname:  "",
				TargetIPAddress: "10.10.10.1",
			},
			wantErr: false,
		},
		{
			name: "omitted-loadbalancer",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Annotations: map[string]string{
							HostnameAnnotationKey:    "test.test.example.com",
							HealthCheckAnnotationKey: "enable",
							zoneAnnotationKey:        "test",
						},
						UID: "aaa",
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "10.10.10.1"},
							},
						},
					},
				},
			},
			want: UpsertRecordSetOpt{
				Hostname:        "test.test.example.com",
				Type:            "A",
				Identifier:      "test/test/aaa",
				HealthCheckID:   "",
				HostedZoneID:    "test",
				Weight:          1,
				TTL:             10,
				Alias:           false,
				TargetHostname:  "",
				TargetIPAddress: "10.10.10.1",
			},
			wantErr: false,
		},
		{
			name: "externalName",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Annotations: map[string]string{
							HostnameAnnotationKey:      "test.test.example.com",
							aliasAnnotationKey:         "true",
							ttlAnnotationKey:           "10",
							healhCheckIdAnnotationKey:  "",
							weightAnnotationKey:        "1",
							setIdentifierAnnotationKey: "test/test",
							recordTypeAnnotationKey:    "A",
							HealthCheckAnnotationKey:   "enable",
							zoneAnnotationKey:          "test",
						},
						UID: "aaa",
					},
					Spec: corev1.ServiceSpec{
						Type:         corev1.ServiceTypeExternalName,
						ExternalName: "test.release.example.com",
					},
				},
			},
			want: UpsertRecordSetOpt{
				Hostname:        "test.test.example.com",
				Type:            "A",
				Identifier:      "test/test",
				HealthCheckID:   "",
				HostedZoneID:    "test",
				Weight:          1,
				TTL:             10,
				Alias:           true,
				TargetHostname:  "test.release.example.com",
				TargetIPAddress: "",
			},
			wantErr: false,
		},
		{
			name: "omitted-externalName",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Annotations: map[string]string{
							HostnameAnnotationKey:    "test.test.example.com",
							HealthCheckAnnotationKey: "enable",
							zoneAnnotationKey:        "test",
						},
						UID: "aaa",
					},
					Spec: corev1.ServiceSpec{
						Type:         corev1.ServiceTypeExternalName,
						ExternalName: "test.release.example.com",
					},
				},
			},
			want: UpsertRecordSetOpt{
				Hostname:        "test.test.example.com",
				Type:            "A",
				Identifier:      "test/test/aaa",
				HealthCheckID:   "",
				HostedZoneID:    "test",
				Weight:          1,
				TTL:             10,
				Alias:           true,
				TargetHostname:  "test.release.example.com",
				TargetIPAddress: "",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toUpsertRecordSetOpt(tt.args.svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("toUpsertRecordSetOpt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toUpsertRecordSetOpt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsure(t *testing.T) {
	type args struct {
		svc *corev1.Service
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "omitted-loadbalancer",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Annotations: map[string]string{
							HostnameAnnotationKey:    "omitted-lb.test.takutakahashi.dev",
							HealthCheckAnnotationKey: "enable",
							zoneAnnotationKey:        "Z09261522C0IVI11TUTK7",
						},
					},
					Spec: corev1.ServiceSpec{
						Type: corev1.ServiceTypeLoadBalancer,
					},
					Status: corev1.ServiceStatus{
						LoadBalancer: corev1.LoadBalancerStatus{
							Ingress: []corev1.LoadBalancerIngress{
								{IP: "10.10.10.1"},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "omitted-externalName",
			args: args{
				svc: &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Annotations: map[string]string{
							HostnameAnnotationKey:    "test1.test.takutakahashi.dev",
							HealthCheckAnnotationKey: "enable",
							zoneAnnotationKey:        "Z09261522C0IVI11TUTK7",
						},
					},
					Spec: corev1.ServiceSpec{
						Type:         corev1.ServiceTypeExternalName,
						ExternalName: "external-route53.test.takutakahashi.dev",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Ensure(tt.args.svc); (err != nil) != tt.wantErr {
				t.Errorf("Ensure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
