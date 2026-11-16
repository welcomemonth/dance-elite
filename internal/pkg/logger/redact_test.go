package logger

import "testing"

func TestRedactText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "id_card_embedded",
			in:   "证件号 11010119900307123X 校验失败",
			want: "证件号 110***********123X 校验失败",
		},
		{
			name: "id_card_lowercase_x",
			in:   "id=11010119900307123x",
			want: "id=110***********123X",
		},
		{
			name: "multiple",
			in:   "11010119900307123X / 32010119880101123X",
			want: "110***********123X / 320***********123X",
		},
		{
			name: "no_match",
			in:   "普通错误信息没有证件号",
			want: "普通错误信息没有证件号",
		},
		{
			name: "phone_embedded",
			in:   "手机号 13812345678 无法接收验证码",
			want: "手机号 138****5678 无法接收验证码",
		},
		{
			name: "adjacent_phones",
			in:   "13812345678,13912345678",
			want: "138****5678,139****5678",
		},
		{
			name: "order_no_not_treated_as_phone",
			in:   "订单 202611162131598412345678 支付失败",
			want: "订单 202611162131598412345678 支付失败",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RedactText(tc.in); got != tc.want {
				t.Errorf("RedactText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestRedactIDNumber(t *testing.T) {
	if got := RedactIDNumber(""); got != "" {
		t.Errorf("empty should stay empty, got %q", got)
	}
	if got := RedactIDNumber("11010119900307123X"); got != "110***********123X" {
		t.Errorf("RedactIDNumber = %q", got)
	}
}
