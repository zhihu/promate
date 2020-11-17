package prometheus

import (
	"reflect"
	"testing"
)

func TestConvertGraphiteTarget(t *testing.T) {
	type args struct {
		query    string
		terminal bool
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 LabelFilters
	}{
		{
			name: "a.b.c",
			args: args{
				query:    "a.b.c",
				terminal: false,
			},
			want: "a",
			want1: LabelFilters{
				{
					Label: "__a_g1__",
					Value: "b",
				},
				{
					Label: "__a_g2__",
					Value: "c",
				},
			},
		},
		{
			name: "a.b.c with terminal",
			args: args{
				query:    "a.b.c",
				terminal: true,
			},
			want: "a",
			want1: LabelFilters{
				{
					Label: "__a_g1__",
					Value: "b",
				},
				{
					Label: "__a_g2__",
					Value: "c",
				},
				{
					Label: "__a_g3__",
					Value: "",
				},
			},
		},
		{
			name: "a.*.c",
			args: args{
				query:    "a.*.c",
				terminal: false,
			},
			want: "a",
			want1: LabelFilters{
				{
					Label: "__a_g2__",
					Value: "c",
				},
			},
		},
		{
			name: "a-a.b",
			args: args{
				query:    "a-a.*.c",
				terminal: false,
			},
			want: "a_a",
			want1: LabelFilters{
				{
					Label: "__a_a_g2__",
					Value: "c",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := ConvertGraphiteTarget(tt.args.query, tt.args.terminal)
			if got != tt.want {
				t.Errorf("ConvertGraphiteTarget() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("ConvertGraphiteTarget() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestConvertPrometheusMetric(t *testing.T) {
	type args struct {
		name   string
		metric map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "a.b.c",
			args: args{
				name: "a",
				metric: map[string]string{
					"__a_g1__": "b",
					"__a_g2__": "c",
				},
			},
			want: "a.b.c",
		},
		{
			name: "unknown name",
			args: args{
				name: "a",
				metric: map[string]string{
					"__name__": "unknown",
					"__a_g1__": "b",
					"__a_g2__": "c",
				},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertPrometheusMetric(tt.args.name, tt.args.metric); got != tt.want {
				t.Errorf("ConvertPrometheusMetric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertQueryLabel(t *testing.T) {
	type args struct {
		query string
	}
	tests := []struct {
		name       string
		args       args
		wantPrefix string
		wantLabel  string
		wantFast   bool
	}{
		{
			name: "a.b.*",
			args: args{
				query: "a.b.*",
			},
			wantPrefix: "a.b.",
			wantLabel:  "__a_g2__",
			wantFast:   false,
		},
		{
			name: "a.* fast",
			args: args{
				query: "a.*",
			},
			wantPrefix: "a.",
			wantLabel:  "__a_g1__",
			wantFast:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPrefix, gotLabel, gotFast := ConvertQueryLabel(tt.args.query)
			if gotPrefix != tt.wantPrefix {
				t.Errorf("ConvertQueryLabel() gotPrefix = %v, want %v", gotPrefix, tt.wantPrefix)
			}
			if gotLabel != tt.wantLabel {
				t.Errorf("ConvertQueryLabel() gotLabel = %v, want %v", gotLabel, tt.wantLabel)
			}
			if gotFast != tt.wantFast {
				t.Errorf("ConvertQueryLabel() gotFast = %v, want %v", gotFast, tt.wantFast)
			}
		})
	}
}

func TestLabelFilters_Build(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name         string
		l            LabelFilters
		args         args
		wantSelector string
	}{
		{
			name: "success",
			l: LabelFilters{
				{
					Label: "g1",
					Value: "v1",
				},
				{
					Label:    "g2",
					Value:    "v2",
					IsRegexp: true,
				},
				{
					Label:      "g3",
					Value:      "v3",
					IsNegative: true,
				},
			},
			args: args{
				name: "name",
			},
			wantSelector: `{__name__="name",g1="v1",g2=~"v2",g3!="v3"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotSelector := tt.l.Build(tt.args.name); gotSelector != tt.wantSelector {
				t.Errorf("Build() = %v, want %v", gotSelector, tt.wantSelector)
			}
		})
	}
}

func Test_labelName(t *testing.T) {
	type args struct {
		name string
		i    int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "a",
			args: args{
				name: "a",
				i:    1,
			},
			want: "__a_g1__",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := labelName(tt.args.name, tt.args.i); got != tt.want {
				t.Errorf("labelName() = %v, want %v", got, tt.want)
			}
		})
	}
}
