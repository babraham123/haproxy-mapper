package main

import (
	"testing"
)

func TestCombiningProcessor(t *testing.T) {
	type in struct{ cidr, value string }
	for _, test := range []struct {
		desc     string
		blocks   []in
		expected []string
	}{
		{
			desc:     "nothing",
			blocks:   []in{},
			expected: []string{},
		},
		{
			desc:     "single block passes through",
			blocks:   []in{{"1.2.3.4/32", "X"}},
			expected: []string{"1.2.3.4/32 X"},
		},
		{
			desc:     "adjacent halves become one",
			blocks:   []in{{"1.2.3.0/25", "X"}, {"1.2.3.128/25", "X"}},
			expected: []string{"1.2.3.0/24 X"},
		},
		{
			desc:     "adjacent /32s become /31",
			blocks:   []in{{"1.2.3.0/32", "X"}, {"1.2.3.1/32", "X"}},
			expected: []string{"1.2.3.0/31 X"},
		},
		{
			desc: "many adjacent blocks become one",
			blocks: []in{
				{"1.0.0.0/16", "X"}, {"1.1.0.0/16", "X"}, {"1.2.0.0/15", "X"},
				{"1.4.0.0/14", "X"}, {"1.8.0.0/13", "X"}, {"1.16.0.0/12", "X"},
				{"1.32.0.0/11", "X"}, {"1.64.0.0/10", "X"}, {"1.128.0.0/9", "X"},
			},
			expected: []string{"1.0.0.0/8 X"},
		},
		{
			desc:     "whole IPv4 space",
			blocks:   []in{{"0.0.0.0/1", "X"}, {"128.0.0.0/1", "X"}},
			expected: []string{"0.0.0.0/0 X"},
		},
		{
			desc:     "unaligned run is split into the fewest CIDRs",
			blocks:   []in{{"1.0.255.255/32", "X"}, {"1.1.0.0/16", "X"}},
			expected: []string{"1.0.255.255/32 X", "1.1.0.0/16 X"},
		},
		{
			desc: "run that can't be reduced",
			blocks: []in{
				{"81.2.69.142/31", "EU"}, {"81.2.69.144/28", "EU"},
				{"81.2.69.160/27", "EU"}, {"81.2.69.192/28", "EU"},
			},
			expected: []string{"81.2.69.142/31 EU", "81.2.69.144/28 EU", "81.2.69.160/27 EU", "81.2.69.192/28 EU"},
		},
		{
			desc:     "value change ends the run, last run is kept",
			blocks:   []in{{"1.0.0.0/25", "AU"}, {"1.0.0.128/25", "AU"}, {"1.0.1.0/24", "CN"}},
			expected: []string{"1.0.0.0/24 AU", "1.0.1.0/24 CN"},
		},
		{
			desc:     "gap ends the run",
			blocks:   []in{{"1.0.0.0/24", "X"}, {"1.0.2.0/24", "X"}},
			expected: []string{"1.0.0.0/24 X", "1.0.2.0/24 X"},
		},
		{
			desc:     "IPv4 then IPv6 with the same value",
			blocks:   []in{{"255.255.255.255/32", "X"}, {"::/128", "X"}},
			expected: []string{"255.255.255.255/32 X", "::/128 X"},
		},
		{
			desc:     "adjacent IPv6 halves become one",
			blocks:   []in{{"2001:db8::/33", "X"}, {"2001:db8:8000::/33", "X"}},
			expected: []string{"2001:db8::/32 X"},
		},
		{
			desc:     "single IPv6 address",
			blocks:   []in{{"2001:db8::1/128", "X"}},
			expected: []string{"2001:db8::1/128 X"},
		},
		{
			desc:     "unaligned IPv6 run",
			blocks:   []in{{"2001:db8::1/128", "X"}, {"2001:db8::2/127", "X"}},
			expected: []string{"2001:db8::1/128 X", "2001:db8::2/127 X"},
		},
		{
			desc:     "IPv6 run ending at the last address",
			blocks:   []in{{"ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffe/128", "X"}, {"ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128", "X"}},
			expected: []string{"ffff:ffff:ffff:ffff:ffff:ffff:ffff:fffe/127 X"},
		},
	} {
		combiner := CombiningProcessorCreate()
		out := &recorder{}
		combiner.AddReceiver(out)
		for _, block := range test.blocks {
			if err := combiner.Receive("test", mustBlock(t, block.cidr, block.value)); err != nil {
				t.Fatalf("%s: %v", test.desc, err)
			}
		}
		if err := combiner.Done("test"); err != nil {
			t.Fatalf("%s: %v", test.desc, err)
		}
		assertLines(t, test.desc, out.lines, test.expected)
		if !out.done {
			t.Errorf("%s: Done not passed on", test.desc)
		}
	}
}
