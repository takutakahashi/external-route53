package dns

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	gomock "github.com/golang/mock/gomock"

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
		name     string
		args     args
		beforeDo func() (Dns, *gomock.Controller)
		wantErr  bool
	}{
		{
			name: "ok",
			args: args{ro: ROs[0]},
			beforeDo: func() (Dns, *gomock.Controller) {
				ro := ROs[0]
				changes := []*route53.Change{
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name:            aws.String(ro.Hostname),
							AliasTarget:     nil,
							ResourceRecords: []*route53.ResourceRecord{{Value: aws.String(ro.TargetIPAddress)}},
							SetIdentifier:   aws.String(ro.Identifier),
							Weight:          aws.Int64(int64(ro.Weight)),
							Type:            aws.String(ro.Type),
							TTL:             aws.Int64(int64(ro.TTL)),
						},
					},
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String(fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)),
							ResourceRecords: []*route53.ResourceRecord{
								{Value: aws.String("\"set by external-route53\"")},
							},
							SetIdentifier: aws.String(ro.Identifier),
							Weight:        aws.Int64(int64(ro.Weight)),
							Type:          aws.String("TXT"),
							TTL:           aws.Int64(300),
						},
					},
				}

				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ChangeResourceRecordSets(
					&route53.ChangeResourceRecordSetsInput{
						HostedZoneId: aws.String(ro.HostedZoneID),
						ChangeBatch: &route53.ChangeBatch{
							Comment: aws.String("change from external-route53"),
							Changes: changes,
						},
					},
				).Return(
					nil,
					nil,
				).Times(1)

				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:    aws.String(ro.HostedZoneID),
						StartRecordName: aws.String(fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{},
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, controller := tt.beforeDo()
			defer controller.Finish()
			if err := d.ensureRecord(tt.args.ro); (err != nil) != tt.wantErr {
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
		name     string
		args     args
		beforeDo func() (Dns, *gomock.Controller)
		want     bool
		wantErr  bool
	}{
		{
			name: "ok",
			args: args{ro: ROs[0]},
			beforeDo: func() (Dns, *gomock.Controller) {
				ro := ROs[0]
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:          aws.String(ro.HostedZoneID),
						StartRecordIdentifier: aws.String(ro.Identifier),
						StartRecordName:       aws.String(fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)),
						StartRecordType:       aws.String(ro.Type),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(ro.Hostname),
								SetIdentifier: aws.String(ro.Identifier),
							},
						},
					},
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "ok",
			args: args{ro: ROs[1]},
			beforeDo: func() (Dns, *gomock.Controller) {
				ro := ROs[1]
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:          aws.String(ro.HostedZoneID),
						StartRecordIdentifier: aws.String(ro.Identifier),
						StartRecordName:       aws.String(fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)),
						StartRecordType:       aws.String(ro.Type),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(ro.Hostname),
								SetIdentifier: aws.String(ro.Identifier),
							},
						},
					},
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "ng",
			args: args{ro: ROs[2]},
			beforeDo: func() (Dns, *gomock.Controller) {
				ro := ROs[2]
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:          aws.String(ro.HostedZoneID),
						StartRecordIdentifier: aws.String(ro.Identifier),
						StartRecordName:       aws.String(fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)),
						StartRecordType:       aws.String(ro.Type),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String("exists.test.takutakahashi.dev."),
								SetIdentifier: aws.String("/api/v1/namespaces/beta/services/test"),
							},
						},
					},
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
			},
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, controller := tt.beforeDo()
			defer controller.Finish()
			got, err := d.recordExists(tt.args.ro)
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
		name     string
		args     args
		beforeDo func() (Dns, *gomock.Controller)
		wantErr  bool
	}{
		{
			name: "ok",
			args: args{
				ro: ROs[0],
			},
			beforeDo: func() (Dns, *gomock.Controller) {
				ro := ROs[0]

				changes := []*route53.Change{
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name:            aws.String(ro.Hostname),
							AliasTarget:     nil,
							ResourceRecords: []*route53.ResourceRecord{{Value: aws.String(ro.TargetIPAddress)}},
							SetIdentifier:   aws.String(ro.Identifier),
							Weight:          aws.Int64(int64(ro.Weight)),
							Type:            aws.String(ro.Type),
							TTL:             aws.Int64(int64(ro.TTL)),
						},
					},
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String(fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)),
							ResourceRecords: []*route53.ResourceRecord{
								{Value: aws.String("\"set by external-route53\"")},
							},
							SetIdentifier: aws.String(ro.Identifier),
							Weight:        aws.Int64(int64(ro.Weight)),
							Type:          aws.String("TXT"),
							TTL:           aws.Int64(300),
						},
					},
				}

				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ChangeResourceRecordSets(
					&route53.ChangeResourceRecordSetsInput{
						HostedZoneId: aws.String(ro.HostedZoneID),
						ChangeBatch: &route53.ChangeBatch{
							Comment: aws.String("change from external-route53"),
							Changes: changes,
						},
					},
				).Return(
					nil,
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, controller := tt.beforeDo()
			defer controller.Finish()
			if err := d.upsert(tt.args.ro); (err != nil) != tt.wantErr {
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
		name     string
		args     args
		beforeDo func() (Dns, *gomock.Controller)
		wantErr  bool
	}{
		{
			name: "ok",
			args: args{
				ro: ROs[0],
			},
			beforeDo: func() (Dns, *gomock.Controller) {
				ro := ROs[0]

				changes := []*route53.Change{
					{
						Action: aws.String("DELETE"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name:            aws.String(ro.Hostname),
							AliasTarget:     nil,
							ResourceRecords: []*route53.ResourceRecord{{Value: aws.String(ro.TargetIPAddress)}},
							SetIdentifier:   aws.String(ro.Identifier),
							Weight:          aws.Int64(int64(ro.Weight)),
							Type:            aws.String(ro.Type),
							TTL:             aws.Int64(int64(ro.TTL)),
						},
					},
					{
						Action: aws.String("DELETE"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String(fmt.Sprintf("%s%s", ro.TXTPrefix, ro.Hostname)),
							ResourceRecords: []*route53.ResourceRecord{
								{Value: aws.String("\"set by external-route53\"")},
							},
							SetIdentifier: aws.String(ro.Identifier),
							Weight:        aws.Int64(int64(ro.Weight)),
							Type:          aws.String("TXT"),
							TTL:           aws.Int64(300),
						},
					},
				}

				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ChangeResourceRecordSets(
					&route53.ChangeResourceRecordSetsInput{
						HostedZoneId: aws.String(ro.HostedZoneID),
						ChangeBatch: &route53.ChangeBatch{
							Comment: aws.String("change from external-route53"),
							Changes: changes,
						},
					},
				).Return(
					nil,
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, controller := tt.beforeDo()
			defer controller.Finish()
			if err := d.delete(tt.args.ro); (err != nil) != tt.wantErr {
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
		name     string
		args     args
		beforeDo func() (Dns, *gomock.Controller)
		want     UpsertRecordSetOpt
		wantErr  bool
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
							HealthCheckIdAnnotationKey: "",
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
			beforeDo: func() (Dns, *gomock.Controller) {
				txtname := fmt.Sprintf("%s%s", "extr53-", "test.test.example.com")
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:    aws.String("test"),
						StartRecordName: aws.String(txtname),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(txtname),
								SetIdentifier: aws.String("test/test"),
								Type:          aws.String("TXT"),
							},
						},
					},
					nil,
				).Times(1)

				return Dns{client: r53api}, controller
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
				TXTPrefix:       "extr53-",
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
			beforeDo: func() (Dns, *gomock.Controller) {
				txtname := fmt.Sprintf("%s%s", "extr53-", "test.test.example.com")
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:    aws.String("test"),
						StartRecordName: aws.String(txtname),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(txtname),
								SetIdentifier: aws.String("test/test/aaa"),
								Type:          aws.String("TXT"),
							},
						},
					},
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
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
				TXTPrefix:       "extr53-",
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
							HealthCheckIdAnnotationKey: "",
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
			beforeDo: func() (Dns, *gomock.Controller) {
				txtname := fmt.Sprintf("%s%s", "extr53-", "test.test.example.com")
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:    aws.String("test"),
						StartRecordName: aws.String(txtname),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(txtname),
								SetIdentifier: aws.String("test/test"),
								Type:          aws.String("TXT"),
							},
						},
					},
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
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
				TXTPrefix:       "extr53-",
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
			beforeDo: func() (Dns, *gomock.Controller) {
				txtname := fmt.Sprintf("%s%s", "extr53-", "test.test.example.com")
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:    aws.String("test"),
						StartRecordName: aws.String(txtname),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(txtname),
								SetIdentifier: aws.String("test/test/aaa"),
								Type:          aws.String("TXT"),
							},
						},
					},
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
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
				TXTPrefix:       "extr53-",
			},
			wantErr: false,
		},
		{
			name: "elb-on-eks",
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
								{Hostname: "test.elb.ap-northeast-1.amazonaws.com"},
							},
						},
					},
				},
			},
			beforeDo: func() (Dns, *gomock.Controller) {
				txtname := fmt.Sprintf("%s%s", "extr53-", "test.test.example.com")
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:    aws.String("test"),
						StartRecordName: aws.String(txtname),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(txtname),
								SetIdentifier: aws.String("test/test/aaa"),
								Type:          aws.String("TXT"),
							},
						},
					},
					nil,
				).Times(1)
				getElb := func(dnsName string) (zoneId *string, err error) {
					return aws.String("elb-hosted-zone"), nil
				}

				return Dns{client: r53api, getElbCanonicalHostedZoneId: getElb}, controller
			},
			want: UpsertRecordSetOpt{
				Hostname:        "test.test.example.com",
				Type:            "A",
				Identifier:      "test/test/aaa",
				HealthCheckID:   "",
				HostedZoneID:    "test",
				ElbHostedZoneID: "elb-hosted-zone",
				Weight:          1,
				TTL:             10,
				Alias:           true,
				TargetHostname:  "test.elb.ap-northeast-1.amazonaws.com",
				TargetIPAddress: "",
				TXTPrefix:       "extr53-",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, controller := tt.beforeDo()
			defer controller.Finish()
			got, err := d.toUpsertRecordSetOpt(tt.args.svc)
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
		name     string
		args     args
		beforeDo func() (Dns, *gomock.Controller)
		wantErr  bool
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
			beforeDo: func() (Dns, *gomock.Controller) {
				txtname := fmt.Sprintf("%s%s", "extr53-", "omitted-lb.test.takutakahashi.dev")
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:    aws.String("Z09261522C0IVI11TUTK7"),
						StartRecordName: aws.String(txtname),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(txtname),
								SetIdentifier: aws.String("test/test/aaa"),
								Type:          aws.String("TXT"),
							},
						},
					},
					nil,
				).Times(2)

				changes := []*route53.Change{
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name:            aws.String("omitted-lb.test.takutakahashi.dev"),
							AliasTarget:     nil,
							ResourceRecords: []*route53.ResourceRecord{{Value: aws.String("10.10.10.1")}},
							SetIdentifier:   aws.String("test/test/"),
							Weight:          aws.Int64(int64(1)),
							Type:            aws.String("A"),
							TTL:             aws.Int64(int64(10)),
						},
					},
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String(txtname),
							ResourceRecords: []*route53.ResourceRecord{
								{Value: aws.String("\"set by external-route53\"")},
							},
							SetIdentifier: aws.String("test/test/"),
							Weight:        aws.Int64(int64(1)),
							Type:          aws.String("TXT"),
							TTL:           aws.Int64(300),
						},
					},
				}
				r53api.EXPECT().ChangeResourceRecordSets(
					&route53.ChangeResourceRecordSetsInput{
						HostedZoneId: aws.String("Z09261522C0IVI11TUTK7"),
						ChangeBatch: &route53.ChangeBatch{
							Comment: aws.String("change from external-route53"),
							Changes: changes,
						},
					},
				).Return(
					nil,
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
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
			beforeDo: func() (Dns, *gomock.Controller) {
				txtname := "external-route53.test.takutakahashi.dev"
				controller := gomock.NewController(t)
				r53api := NewMockRoute53API(controller)
				r53api.EXPECT().ListResourceRecordSets(
					&route53.ListResourceRecordSetsInput{
						HostedZoneId:    aws.String("Z09261522C0IVI11TUTK7"),
						StartRecordName: aws.String("extr53-test1.test.takutakahashi.dev"),
					},
				).Return(
					&route53.ListResourceRecordSetsOutput{
						ResourceRecordSets: []*route53.ResourceRecordSet{
							{
								Name:          aws.String(txtname),
								SetIdentifier: aws.String("test/test/aaa"),
								Type:          aws.String("TXT"),
							},
						},
					},
					nil,
				).Times(2)

				changes := []*route53.Change{
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String("test1.test.takutakahashi.dev"),
							AliasTarget: &route53.AliasTarget{
								DNSName:              aws.String("external-route53.test.takutakahashi.dev"),
								EvaluateTargetHealth: aws.Bool(true),
								HostedZoneId:         aws.String("Z09261522C0IVI11TUTK7"),
							},
							SetIdentifier: aws.String("test/test/"),
							Weight:        aws.Int64(int64(1)),
							Type:          aws.String("A"),
						},
					},
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String("extr53-test1.test.takutakahashi.dev"),
							ResourceRecords: []*route53.ResourceRecord{
								{Value: aws.String("\"set by external-route53\"")},
							},
							SetIdentifier: aws.String("test/test/"),
							Weight:        aws.Int64(int64(1)),
							Type:          aws.String("TXT"),
							TTL:           aws.Int64(300),
						},
					},
				}
				r53api.EXPECT().ChangeResourceRecordSets(
					&route53.ChangeResourceRecordSetsInput{
						HostedZoneId: aws.String("Z09261522C0IVI11TUTK7"),
						ChangeBatch: &route53.ChangeBatch{
							Comment: aws.String("change from external-route53"),
							Changes: changes,
						},
					},
				).Return(
					nil,
					nil,
				).Times(1)
				return Dns{client: r53api}, controller
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, controller := tt.beforeDo()
			defer controller.Finish()
			if err := d.Ensure(tt.args.svc); (err != nil) != tt.wantErr {
				t.Errorf("Ensure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
