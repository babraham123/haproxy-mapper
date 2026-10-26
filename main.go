package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() (err error) {
	opts := Options{
		Maps:       make(map[string]bool),
		LicenseKey: os.Getenv("MAXMIND_LICENSE_KEY"),
		Fetcher:    HttpFetcherCreate(),
	}
	flag.StringVar(&opts.Outdir, "outdir", "", "Output directory")
	flag.BoolVar(&opts.IPv6, "ipv6", true, "Process IPv6 data, -ipv6=false to disable")
	flag.BoolVar(&opts.All, "all", false, "Include All data")
	flag.StringVar(&opts.AsnDb, "asn-db", "", "MaxMind ASN database file")
	flag.StringVar(&opts.IspDb, "isp-db", "", "MaxMind ISP database file")
	flag.StringVar(&opts.CityDb, "city-db", "", "MaxMind City or Country database file")
	flag.BoolVar(&opts.GeoLite2, "geolite2", false,
		"Download the free MaxMind GeoLite2 databases the selected maps need instead of -asn-db and -city-db,\n"+
			"using the MAXMIND_LICENSE_KEY environment variable")
	countryDir := flag.String("country-dir", "", "Write one <ISO code>.map per country, a CIDR per line, into this directory,\n"+
		"replacing its *.map files; requires -city-db or -geolite2")
	selections := make(map[string]*bool)
	for _, spec := range catalogue {
		selections[spec.flag] = flag.Bool(spec.flag, false, spec.usage)
	}
	flag.Parse()

	for name, on := range selections {
		opts.Maps[name] = *on
	}
	if *countryDir != "" {
		opts.SplitDirs = map[string]string{"country": *countryDir}
	}

	p, err := BuildPipeline(opts)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := p.Close(); err == nil {
			err = closeErr
		}
	}()
	return p.Run()
}
