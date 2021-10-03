package main

import "testing"

func TestParsecRevertRef(t *testing.T) {
	type args struct {
		msg string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "other commit", args: args{msg: "something else"}, want: ""},
		{
			name: "revert commit",
			args: args{msg: "This reverts commit 397747d22bd12cce4bc6bd0aa979a4f8eed3d29a"},
			want: "397747d22bd12cce4bc6bd0aa979a4f8eed3d29a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParsecRevertRef(tt.args.msg); got != tt.want {
				t.Errorf("ParsecRevertRef() = %v, want %v", got, tt.want)
			}
		})
	}
}
