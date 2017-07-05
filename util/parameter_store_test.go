package util

import "testing"

func TestGetSSMParameter(t *testing.T) {
	type args struct {
		parameterName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"t1", args{"/nonexisting/value"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSSMParameter(tt.args.parameterName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSSMParameter(%s) error = %v, wantErr %v", tt.args.parameterName, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetSSMParameter(%s) = %v", tt.args.parameterName, got)
			}
		})
	}
}
