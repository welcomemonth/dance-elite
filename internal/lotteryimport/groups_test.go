package lotteryimport

import "testing"

func TestDistributeGroupSizes(t *testing.T) {
	cases := map[int][]int{
		1:  {1},
		8:  {8},
		12: {12},
		13: {7, 6},
		20: {10, 10},
		24: {12, 12},
		25: {9, 8, 8},
		30: {10, 10, 10},
		35: {12, 12, 11},
		39: {10, 10, 10, 9},
	}
	for n, want := range cases {
		got := distributeGroupSizes(n)
		if len(got) != len(want) {
			t.Fatalf("n=%d got %v want %v", n, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("n=%d got %v want %v", n, got, want)
			}
		}
	}
}

func TestGroupLetter(t *testing.T) {
	cases := map[int]string{0: "A", 1: "B", 25: "Z", 26: "AA", 27: "AB"}
	for index, want := range cases {
		if got := groupLetter(index); got != want {
			t.Errorf("groupLetter(%d) = %q, want %q", index, got, want)
		}
	}
}
