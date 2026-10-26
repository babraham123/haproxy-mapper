package main

import (
	"strings"
)

var cloudflareFeed = FeedSpec{
	id: "cloudflare",
	header: Header{
		general: `#
# IP to Cloudflare mapping
#
# https://www.cloudflare.com/ips-v4
# https://www.cloudflare.com/ips-v6
#
`,
		columns: "# cidr Cloudflare\n",
	},
	blocks: cloudflareBlocks,
}

func cloudflareBlocks(f Fetcher) (Blocks, error) {
	value := "Cloudflare"
	blocks := make(Blocks, 0)
	for _, url := range []string{"https://www.cloudflare.com/ips-v4", "https://www.cloudflare.com/ips-v6"} {
		body, err := fetchBytes(f, url)
		if err != nil {
			return nil, err
		}
		for _, cidr := range strings.Split(string(body), "\n") {
			cidr = strings.TrimSpace(cidr)
			if len(cidr) == 0 {
				continue
			}
			block, err := BlockCreateWithCidr(&cidr, &value)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}
