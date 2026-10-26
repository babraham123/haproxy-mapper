package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Runnable interface {
	Run(ipv4Only bool) error
}

// mapSpec is one map haproxy-mapper can write: the flag that selects it, its
// file in -outdir and where its blocks come from. Exactly one of feed,
// merges, isp or city is set.
type mapSpec struct {
	flag  string
	file  string
	usage string

	feed *FeedSpec
	// The feed is also merged into the provider map.
	inProvider bool
	// The provider map: a merge of the selected inProvider feeds.
	merges bool
	isp    func(*MaxMindIspOrigin, Receiver)
	city   func(*MaxMindCityOrigin, Receiver)
	// The free GeoLite2 edition holding this MaxMind map's data, empty when
	// only the paid databases have it.
	edition string
}

var catalogue = []mapSpec{
	{flag: "aws", file: "ip_to_aws", usage: "Include AWS data", feed: &awsFeed, inProvider: true},
	{flag: "azure", file: "ip_to_azure", usage: "Include Azure data", feed: &azureFeed, inProvider: true},
	{flag: "cloudflare", file: "ip_to_cloudflare", usage: "Include Cloudflare data", feed: &cloudflareFeed, inProvider: true},
	{flag: "fastly", file: "ip_to_fastly", usage: "Include Fastly data", feed: &fastlyFeed, inProvider: true},
	{flag: "google-cloud", file: "ip_to_google_cloud", usage: "Include Google Cloud data", feed: &googleCloudFeed, inProvider: true},
	{flag: "oracle", file: "ip_to_oracle", usage: "Include Oracle data", feed: &oracleFeed, inProvider: true},
	{flag: "provider", file: "ip_to_provider", usage: "Include providers map, merging the selected provider maps", merges: true},
	{flag: "spamhaus", file: "ip_to_spamhaus", usage: "Include spamhaus data", feed: &spamhausFeed},
	{flag: "asn", file: "ip_to_asn", usage: "Include asn map, requires -asn-db, -isp-db or -geolite2",
		isp: (*MaxMindIspOrigin).AddAsnReceiver, edition: "GeoLite2-ASN"},
	{flag: "isp", file: "ip_to_isp", usage: "Include isp map, requires -isp-db",
		isp: (*MaxMindIspOrigin).AddIspReceiver},
	{flag: "location", file: "ip_to_location", usage: "Include location map, requires -city-db or -geolite2",
		city: (*MaxMindCityOrigin).AddLocationReceiver, edition: "GeoLite2-City"},
	{flag: "continent", file: "ip_to_continent", usage: "Include continent map, requires -city-db or -geolite2",
		city: (*MaxMindCityOrigin).AddContinentReceiver, edition: "GeoLite2-Country"},
	{flag: "country", file: "ip_to_country", usage: "Include country map, requires -city-db or -geolite2",
		city: (*MaxMindCityOrigin).AddCountryReceiver, edition: "GeoLite2-Country"},
	{flag: "subdivisions", file: "ip_to_subdivisions", usage: "Include subdivisions map, requires -city-db or -geolite2",
		city: (*MaxMindCityOrigin).AddSubdivisionsReceiver, edition: "GeoLite2-City"},
	{flag: "city", file: "ip_to_city", usage: "Include city map, requires -city-db or -geolite2",
		city: (*MaxMindCityOrigin).AddCityReceiver, edition: "GeoLite2-City"},
}

type Options struct {
	Outdir string
	// Maps to write to Outdir, by flag name.
	Maps map[string]bool
	// Every map, with isp only when the ISP database has ISP data.
	All    bool
	IPv6   bool
	AsnDb  string
	IspDb  string
	CityDb string
	// Download the free GeoLite2 databases instead of AsnDb and CityDb.
	GeoLite2   bool
	LicenseKey string
	// Also write a map's blocks as one pattern file per value into a
	// directory, by flag name. Selects the map's source but not its file.
	SplitDirs map[string]string
	Fetcher   Fetcher
}

// Pipeline is the set of origins wired to their destinations for one run.
type Pipeline struct {
	runnables []Runnable
	ipv4Only  bool
	closers   []func() error
}

// BuildPipeline validates opts, downloads and opens any databases, and wires
// every selected map. Files in Outdir are created here; they're written by Run.
func BuildPipeline(opts Options) (_ *Pipeline, err error) {
	// selected maps get a file in Outdir; needed maps have their source run.
	selected := make(map[string]bool)
	needed := make(map[string]bool)
	for _, spec := range catalogue {
		if opts.Maps[spec.flag] || (opts.All && spec.flag != "isp") {
			selected[spec.flag] = true
			needed[spec.flag] = true
		}
		if opts.SplitDirs[spec.flag] != "" {
			needed[spec.flag] = true
		}
	}

	if err := validate(opts, selected, needed); err != nil {
		return nil, err
	}

	p := &Pipeline{ipv4Only: !opts.IPv6}
	defer func() {
		if err != nil {
			p.Close()
		}
	}()

	asnDb, ispDb, cityDb := opts.AsnDb, opts.IspDb, opts.CityDb
	if opts.GeoLite2 {
		if asnDb, cityDb, err = p.downloadGeoLite2(opts, needed); err != nil {
			return nil, err
		}
	}

	ispAsnDb := ispDb
	if asnDb != "" {
		ispAsnDb = asnDb
	}
	var isp *MaxMindIspOrigin
	if ispAsnDb != "" {
		if isp, err = MaxMindIspOriginCreate(ispAsnDb); err != nil {
			return nil, err
		}
		p.closers = append(p.closers, isp.Close)
		if ispDb != "" && !isp.HaveIspData {
			return nil, errors.New("argument -isp-db specified, but database does not include ISP data")
		}
		if opts.All && isp.HaveIspData {
			selected["isp"] = true
			needed["isp"] = true
		}
		p.runnables = append(p.runnables, isp)
	}

	var city *MaxMindCityOrigin
	if cityDb != "" {
		if city, err = MaxMindCityOriginCreate(cityDb); err != nil {
			return nil, err
		}
		p.closers = append(p.closers, city.Close)
		p.runnables = append(p.runnables, city)
	}

	var merger *MergingProcessor
	if selected["provider"] {
		merger = MergingProcessorCreate("provider")
		dest, err := MapDestinationCreate(filepath.Join(opts.Outdir, "ip_to_provider"))
		if err != nil {
			return nil, err
		}
		merger.AddReceiver(dest)
	}

	for _, spec := range catalogue {
		if !needed[spec.flag] || spec.merges {
			continue
		}

		dests := make([]Receiver, 0)
		if selected[spec.flag] {
			dest, err := MapDestinationCreate(filepath.Join(opts.Outdir, spec.file))
			if err != nil {
				return nil, err
			}
			dests = append(dests, dest)
		}
		if dir := opts.SplitDirs[spec.flag]; dir != "" {
			dest, err := SplitMapDestinationCreate(dir)
			if err != nil {
				return nil, err
			}
			p.closers = append(p.closers, dest.Close)
			dests = append(dests, dest)
		}

		switch {
		case spec.feed != nil:
			feed := FeedCreate(*spec.feed, opts.Fetcher)
			for _, dest := range dests {
				feed.AddReceiver(dest)
			}
			if merger != nil && spec.inProvider {
				feed.AddReceiver(merger)
			}
			p.runnables = append(p.runnables, feed)
		case spec.isp != nil:
			spec.isp(isp, combining(dests))
		case spec.city != nil:
			spec.city(city, combining(dests))
		}
	}

	return p, nil
}

// combining puts a CombiningProcessor in front of dests; MaxMind databases
// split ranges far more finely than the values change.
func combining(dests []Receiver) Receiver {
	combiner := CombiningProcessorCreate()
	for _, dest := range dests {
		combiner.AddReceiver(dest)
	}
	return combiner
}

func validate(opts Options, selected, needed map[string]bool) error {
	cityMaps := make([]string, 0)
	providerFeeds := make([]string, 0)
	for _, spec := range catalogue {
		if spec.inProvider {
			providerFeeds = append(providerFeeds, "-"+spec.flag)
		}
		if spec.city != nil && needed[spec.flag] {
			cityMaps = append(cityMaps, spec.flag)
		}
	}

	if len(needed) == 0 {
		return fmt.Errorf("no outputs specified, see %s -h for help", os.Args[0])
	}
	if len(selected) > 0 && opts.Outdir == "" {
		return errors.New("missing required argument -outdir")
	}
	if selected["provider"] {
		included := false
		for _, spec := range catalogue {
			included = included || (spec.inProvider && selected[spec.flag])
		}
		if !included {
			return fmt.Errorf("argument -provider enabled, but none of %s included", strings.Join(providerFeeds, ", "))
		}
	}

	if opts.GeoLite2 {
		if opts.AsnDb != "" || opts.IspDb != "" || opts.CityDb != "" {
			return errors.New("argument -geolite2 downloads the databases, drop -asn-db, -isp-db and -city-db")
		}
		if opts.LicenseKey == "" {
			return errors.New("argument -geolite2 requires the MAXMIND_LICENSE_KEY environment variable")
		}
		maxmind := false
		for _, spec := range catalogue {
			if (spec.isp == nil && spec.city == nil) || !needed[spec.flag] {
				continue
			}
			if spec.edition == "" {
				return fmt.Errorf("argument -%s enabled, but the free GeoLite2 databases do not include its data", spec.flag)
			}
			maxmind = true
		}
		if !maxmind {
			return errors.New("argument -geolite2 provided, but no associated maps included")
		}
		return nil
	}

	if opts.AsnDb != "" && opts.IspDb != "" {
		return errors.New("-asn-db and -isp-db both specified, use -isp-db")
	} else if needed["isp"] && opts.IspDb == "" {
		return errors.New("argument -isp enabled, but -isp-db not specified")
	} else if needed["asn"] && opts.IspDb == "" && opts.AsnDb == "" {
		return errors.New("argument -asn enabled, but neither -asn-db nor -isp-db specified")
	} else if opts.IspDb != "" && !needed["isp"] && !needed["asn"] {
		return errors.New("argument -isp-db provided, but no associated maps included")
	} else if opts.AsnDb != "" && !needed["asn"] {
		return errors.New("argument -asn-db provided, but no associated maps included")
	}

	if opts.CityDb != "" && len(cityMaps) == 0 {
		return errors.New("argument -city-db provided, but no associated maps included")
	} else if opts.CityDb == "" && len(cityMaps) > 0 {
		return fmt.Errorf("the %s map needs -city-db or -geolite2", cityMaps[0])
	}

	return nil
}

// downloadGeoLite2 fetches the smallest free databases that cover the needed
// maps, returning the ASN and City/Country database paths.
func (p *Pipeline) downloadGeoLite2(opts Options, needed map[string]bool) (asnDb, cityDb string, err error) {
	dir, err := os.MkdirTemp("", "haproxy-mapper-")
	if err != nil {
		return "", "", err
	}
	p.closers = append(p.closers, func() error { return os.RemoveAll(dir) })

	cityEdition := ""
	for _, spec := range catalogue {
		if !needed[spec.flag] {
			continue
		}
		if spec.isp != nil && asnDb == "" {
			if asnDb, err = DownloadGeoLite2(opts.Fetcher, spec.edition, opts.LicenseKey, dir); err != nil {
				return "", "", err
			}
		}
		// City is a superset of Country.
		if spec.city != nil && cityEdition != "GeoLite2-City" {
			cityEdition = spec.edition
		}
	}
	if cityEdition != "" {
		if cityDb, err = DownloadGeoLite2(opts.Fetcher, cityEdition, opts.LicenseKey, dir); err != nil {
			return "", "", err
		}
	}
	return asnDb, cityDb, nil
}

// Run runs every origin concurrently and waits for all of them.
func (p *Pipeline) Run() error {
	errs := make(chan error, len(p.runnables))
	for _, runnable := range p.runnables {
		go func(r Runnable) {
			errs <- r.Run(p.ipv4Only)
		}(runnable)
	}

	all := make([]error, 0)
	for range p.runnables {
		if err := <-errs; err != nil {
			all = append(all, err)
		}
	}
	return errors.Join(all...)
}

// Close releases databases and removes downloads and staging directories.
func (p *Pipeline) Close() error {
	all := make([]error, 0)
	for i := len(p.closers) - 1; i >= 0; i-- {
		if err := p.closers[i](); err != nil {
			all = append(all, err)
		}
	}
	p.closers = nil
	return errors.Join(all...)
}
