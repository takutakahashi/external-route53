package healthcheck

import (
	"testing"

	"github.com/google/uuid"
	route53v1 "github.com/takutakahashi/external-route53/api/v1"
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
