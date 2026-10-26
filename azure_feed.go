package main

import (
	"errors"
	"regexp"
)

type azureValues struct {
	Values []struct {
		Properties struct {
			Region          string   `json:"region"`
			Platform        string   `json:"platform"`
			SystemService   string   `json:"systemService"`
			AddressPrefixes []string `json:"addressPrefixes"`
		} `json:"properties"`
	} `json:"values"`
}

const azureDownloadPage = "https://www.microsoft.com/en-us/download/confirmation.aspx?id=56519"

var azureFeed = FeedSpec{
	id: "azure",
	header: Header{
		general: `#
# IP to Azure mapping
#
# https://www.microsoft.com/en-us/download/confirmation.aspx?id=56519 (hack)
#
`,
		columns: "# cidr Azure/service/region\n",
	},
	blocks: azureBlocks,
}

func azureBlocks(f Fetcher) (Blocks, error) {
	// WARNING: hack incoming... Azure doesn't have a non-authenticated way to
	// grab its list of IP addresses via an api call, but you can visit a
	// webpage to get the "current" url of said data. This is downloading that
	// page and using a regex to pull out the URL we're after. Ugly, but it
	// beats requiring an Azure account for auth... Going to go with this until
	// it proves to flakey or something...
	page, err := fetchBytes(f, azureDownloadPage)
	if err != nil {
		return nil, err
	}

	r := regexp.MustCompile(`click here to download.*href="(?P<url>[^"]+)"`)
	matches := r.FindSubmatch(page)
	if len(matches) != 2 {
		return nil, errors.New("Failed to find the download url (hacky)")
	}

	values := azureValues{}
	if err := fetchJSON(f, string(matches[1]), &values); err != nil {
		return nil, err
	}

	blocks := make(Blocks, 0)
	for _, value := range values.Values {
		if value.Properties.SystemService == "" || value.Properties.Region == "" {
			// If we don't have those fields this is just garbage/duplicate data
			continue
		}
		info := "Azure/" + value.Properties.SystemService + "/" + value.Properties.Region
		for _, cidr := range value.Properties.AddressPrefixes {
			block, err := BlockCreateWithCidr(&cidr, &info)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		}
	}

	return blocks, nil
}
