package lotteryimport

import "testing"

func TestDistributeGroupSizes(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want []int
	}{
		// 边界：n<=14 时 n/15+1=1，n/10<=1，取 1 组
		{"1人", 1, []int{1}},
		{"8人", 8, []int{8}},
		{"12人", 12, []int{12}},
		{"13人", 13, []int{13}},
		{"14人", 14, []int{14}},
		// 15 起进入 2 组
		{"15人", 15, []int{8, 7}},
		{"19人", 19, []int{10, 9}},
		{"20人", 20, []int{10, 10}},
		{"24人", 24, []int{12, 12}},
		{"25人", 25, []int{13, 12}},
		{"29人", 29, []int{15, 14}},
		// 30 起进入 3 组
		{"30人", 30, []int{10, 10, 10}},
		{"31人", 31, []int{11, 10, 10}},
		{"35人", 35, []int{12, 12, 11}},
		{"39人", 39, []int{13, 13, 13}},
		// 40 起进入 4 组
		{"40人", 40, []int{10, 10, 10, 10}},
		{"42人", 42, []int{11, 11, 10, 10}},
		{"45人", 45, []int{12, 11, 11, 11}},
		{"49人", 49, []int{13, 12, 12, 12}},
		// 50 起进入 5 组
		{"51人", 51, []int{11, 10, 10, 10, 10}},
		{"55人", 55, []int{11, 11, 11, 11, 11}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := distributeGroupSizes(tc.n)
			if len(got) != len(tc.want) {
				t.Fatalf("n=%d got %v want %v", tc.n, got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("n=%d got %v want %v", tc.n, got, tc.want)
				}
			}
		})
	}
}

// TestDistributeGroupSizes_Invariants 在较大范围内验证分组结果满足核心不变量：
//  1. 各组人数之和等于 n；
//  2. 各组人数差不超过 1；
//  3. 组数符合 max(n/10, n/15+1)。
func TestDistributeGroupSizes_Invariants(t *testing.T) {
	for n := 1; n <= 500; n++ {
		sizes := distributeGroupSizes(n)
		if len(sizes) == 0 {
			t.Fatalf("n=%d got empty", n)
		}
		sum := 0
		min, max := sizes[0], sizes[0]
		for _, s := range sizes {
			sum += s
			if s < min {
				min = s
			}
			if s > max {
				max = s
			}
		}
		if sum != n {
			t.Fatalf("n=%d sum=%d sizes=%v", n, sum, sizes)
		}
		if max-min > 1 {
			t.Fatalf("n=%d sizes=%v max-min=%d", n, sizes, max-min)
		}
		want := n / 10
		if alt := n/15 + 1; alt > want {
			want = alt
		}
		if len(sizes) != want {
			t.Fatalf("n=%d groups=%d want %d", n, len(sizes), want)
		}
	}
}

func TestDistributeGroupSizes_NonPositive(t *testing.T) {
	for _, n := range []int{0, -1, -10} {
		if got := distributeGroupSizes(n); got != nil {
			t.Errorf("n=%d got %v want nil", n, got)
		}
	}
}

func TestGroupLetter(t *testing.T) {
	cases := []struct {
		index int
		want  string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{701, "ZZ"},
		{702, "AAA"},
	}
	for _, tc := range cases {
		if got := groupLetter(tc.index); got != tc.want {
			t.Errorf("groupLetter(%d) = %q, want %q", tc.index, got, tc.want)
		}
	}
}
