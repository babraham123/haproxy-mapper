package main

import (
	"sort"
	"strings"
)

type oraclePublicIpRanges struct {
	Regions []struct {
		Region string `json:"region"`
		Cidrs  []struct {
			Cidr string   `json:"cidr"`
			Tags []string `json:"tags"`
		} `json:"cidrs"`
	} `json:"regions"`
}

var oracleFeed = FeedSpec{
	id: "oracle",
	header: Header{
		general: `#
# IP to Oracle mapping
#
# https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json
#
`,
		columns: "# cidr Oracle/service[,service]*/region\n",
	},
	blocks: oracleBlocks,
}

func oracleBlocks(f Fetcher) (Blocks, error) {
	ranges := oraclePublicIpRanges{}
	if err := fetchJSON(f, "https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json", &ranges); err != nil {
		return nil, err
	}

	blocks := make(Blocks, 0)
	for _, region := range ranges.Regions {
		for _, cidr := range region.Cidrs {
			// Multiple tags for a given cidr, best we can do is sort and comma seperate them...
			sort.Strings(cidr.Tags)
			value := "Oracle/" + strings.Join(cidr.Tags, ",") + "/" + region.Region
			block, err := BlockCreateWithCidr(&cidr.Cidr, &value)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}
