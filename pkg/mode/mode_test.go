/*
Determines the Execution Modes supported by the application.
  - adhoc: the application will poll the k8s API once and then print and report (if configured) its findings
  - periodic: the application will poll the k8s API on an interval (polling-interval-seconds) and report (if configured) its findings
*/
package mode

import "testing"

func TestParseMode(t *testing.T) {
	type args struct {
		userStr string
	}
	tests := []struct {
		name string
		args args
		want Mode
	}{
		{
			name: "adhoc",
			args: args{
				userStr: "adhoc",
			},
			want: AdHoc,
		},
		{
			name: "periodic",
			args: args{
				userStr: "periodic",
			},
			want: PeriodicPolling,
		},
		{
			name: "invalid",
			args: args{
				userStr: "invalid",
			},
			want: AdHoc,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseMode(tt.args.userStr); got != tt.want {
				t.Errorf("ParseMode() = %v, want %v", got, tt.want)
			}
		})
	}
}
