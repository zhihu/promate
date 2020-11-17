package prometheus

import (
	"testing"
)

func TestCovertMateQuery(t *testing.T) {
	type args struct {
		query    string
		terminal bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: `sum`,
			args: args{
				query:    `sum(rate(a.b.c.d)) by (g1,g2)`,
				terminal: false,
			},
			want:    `sum(rate(a{__a_g1__="b", __a_g2__="c", __a_g3__="d"})) by (__a_g1__, __a_g2__)`,
			wantErr: false,
		},
		{
			name: `sum with duration`,
			args: args{
				query:    `sum(rate(a.b.c.d[5m])) by (g1,g2)`,
				terminal: false,
			},
			want:    `sum(rate(a{__a_g1__="b", __a_g2__="c", __a_g3__="d"}[5m])) by (__a_g1__, __a_g2__)`,
			wantErr: false,
		},
		{
			name: `sum with char range`,
			args: args{
				query:    `sum(rate(a.[bc][cd].d)) by (g1,g2)`,
				terminal: false,
			},
			want:    `sum(rate(a{__a_g1__=~"[bc][cd]", __a_g2__="d"})) by (__a_g1__, __a_g2__)`,
			wantErr: false,
		},
		{
			name: `sum with wildcard`,
			args: args{
				query:    `sum(rate(a.b*.c.d)) by (g1,g2)`,
				terminal: false,
			},
			want:    `sum(rate(a{__a_g1__=~"b[^.]*", __a_g2__="c", __a_g3__="d"})) by (__a_g1__, __a_g2__)`,
			wantErr: false,
		},
		{
			name: `sum with value list`,
			args: args{
				query:    `sum(rate(a.{b,c}.c.d)) by (g1,g2)`,
				terminal: false,
			},
			want:    `sum(rate(a{__a_g1__=~"(b|c)", __a_g2__="c", __a_g3__="d"})) by (__a_g1__, __a_g2__)`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CovertMateQuery(tt.args.query, tt.args.terminal)
			if (err != nil) != tt.wantErr {
				t.Errorf("CovertMateQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CovertMateQuery() got = %v, want %v", got, tt.want)
			}
		})
	}
}
