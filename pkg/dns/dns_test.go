package dns

import (
	"testing"
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
