package main

import (
	"fmt"
	"sort"
)

// FeedSpec describes a published list of IP ranges: the map header and how to
// turn the fetched documents into blocks. Everything else, ordering, IPv6
// filtering and the receiver protocol, is the Feed's job.
type FeedSpec struct {
	id     string
	header Header
	blocks func(f Fetcher) (Blocks, error)
}

// Feed is the origin for a FeedSpec. It emits the header, then the spec's
// blocks in sorted order, then Done.
type Feed struct {
	spec    FeedSpec
	fetcher Fetcher
	Emitter
}

func FeedCreate(spec FeedSpec, fetcher Fetcher) *Feed {
	return &Feed{
		spec:    spec,
		fetcher: fetcher,
		Emitter: Emitter{
			id: spec.id,
		},
	}
}

func (f *Feed) Run(ipv4Only bool) error {
	if err := f.Header(f.spec.header); err != nil {
		return err
	}

	blocks, err := f.spec.blocks(f.fetcher)
	if err != nil {
		return fmt.Errorf("%s: %w", f.spec.id, err)
	}

	sort.Sort(blocks)

	for _, block := range blocks {
		if ipv4Only && block.net.IP.To4() == nil {
			continue
		}
		if err := f.Emit(block); err != nil {
			return err
		}
	}

	return f.Done()
}
