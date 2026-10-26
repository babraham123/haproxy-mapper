package main

import (
	"fmt"
	"strings"
)

var spamhausFeed = FeedSpec{
	id: "spamhaus",
	header: Header{
		general: `#
# IP to Spamhaus mapping
#
# https://www.spamhaus.org/drop/drop.lasso
# https://www.spamhaus.org/drop/edrop.lasso
# https://www.spamhaus.org/drop/dropv6.txt
#
`,
		columns: "# cidr Spamhaus/id\n",
	},
	blocks: spamhausBlocks,
}

func spamhausBlocks(f Fetcher) (Blocks, error) {
	blocks := make(Blocks, 0)
	for _, url := range []string{
		"https://www.spamhaus.org/drop/drop.lasso",
		"https://www.spamhaus.org/drop/edrop.lasso",
		"https://www.spamhaus.org/drop/dropv6.txt",
	} {
		body, err := fetchBytes(f, url)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(body), "\n") {
			if len(line) == 0 || line[0] == ';' {
				continue
			}
			pieces := strings.Split(line, " ; ")
			if len(pieces) != 2 {
				return nil, fmt.Errorf("%s: unexpected droplist line %q", url, line)
			}
			value := "Spamhaus" + "/" + pieces[1]
			block, err := BlockCreateWithCidr(&pieces[0], &value)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}
