package main

type fastlyPublicIpList struct {
	Addresses     []string `json:"addresses"`
	Ipv6Addresses []string `json:"ipv6_addresses"`
}

var fastlyFeed = FeedSpec{
	id: "fastly",
	header: Header{
		general: `#
# IP to Fastly mapping
#
# https://api.fastly.com/public-ip-list
#
`,
		columns: "# cidr Fastly\n",
	},
	blocks: fastlyBlocks,
}

func fastlyBlocks(f Fetcher) (Blocks, error) {
	ranges := fastlyPublicIpList{}
	if err := fetchJSON(f, "https://api.fastly.com/public-ip-list", &ranges); err != nil {
		return nil, err
	}

	value := "Fastly"
	blocks := make(Blocks, 0)
	for _, cidr := range append(ranges.Addresses, ranges.Ipv6Addresses...) {
		block, err := BlockCreateWithCidr(&cidr, &value)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}
