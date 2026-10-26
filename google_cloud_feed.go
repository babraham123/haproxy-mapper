package main

type gcIpRanges struct {
	Prefixes []struct {
		Ipv4Prefix string `json:"ipv4Prefix"`
		Ipv6Prefix string `json:"ipv6Prefix"`
		Service    string `json:"service"`
		Scope      string `json:"scope"`
	} `json:"prefixes"`
}

var googleCloudFeed = FeedSpec{
	id: "google_cloud",
	header: Header{
		general: `#
# IP to Google Cloud mapping
#
# https://www.gstatic.com/ipranges/cloud.json
#
# Note: All "service" values appear to be "Google Cloud"
#
`,
		columns: "# cidr GC/service/scope\n",
	},
	blocks: googleCloudBlocks,
}

func googleCloudBlocks(f Fetcher) (Blocks, error) {
	ranges := gcIpRanges{}
	if err := fetchJSON(f, "https://www.gstatic.com/ipranges/cloud.json", &ranges); err != nil {
		return nil, err
	}

	blocks := make(Blocks, 0)
	for _, prefix := range ranges.Prefixes {
		value := "GC/" + prefix.Service + "/" + prefix.Scope
		cidr := prefix.Ipv4Prefix
		if cidr == "" {
			cidr = prefix.Ipv6Prefix
		}
		block, err := BlockCreateWithCidr(&cidr, &value)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}
