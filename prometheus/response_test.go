package prometheus

import "testing"

func TestMatrixPair_UnmarshalJSON(t *testing.T) {
	type fields struct {
		Timestamp float64
		Value     float64
	}
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				Timestamp: 1590249600,
				Value:     1,
			},
			args: args{
				[]byte(`[1590249600,"1"]`),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MatrixPair{
				Timestamp: tt.fields.Timestamp,
				Value:     tt.fields.Value,
			}
			if err := m.UnmarshalJSON(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
