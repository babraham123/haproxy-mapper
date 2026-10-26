package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maxmind/mmdbwriter/mmdbtype"
)

var feedFixtures = fixtures{
	"https://ip-ranges.amazonaws.com/ip-ranges.json": []byte(`{
		"prefixes": [
			{"ip_prefix": "3.5.140.0/22", "region": "ap-northeast-2", "service": "AMAZON"},
			{"ip_prefix": "3.5.140.0/22", "region": "ap-northeast-2", "service": "S3"},
			{"ip_prefix": "15.230.56.104/31", "region": "us-east-1", "service": "AMAZON"}
		],
		"ipv6_prefixes": [
			{"ipv6_prefix": "2600:1f14::/35", "region": "us-west-2", "service": "EC2"}
		]
	}`),
	azureDownloadPage: []byte(`<html><body>
<p>If the download doesn't start, click here to download manually: <a href="https://download.example/ServiceTags_Public.json">ServiceTags</a></p>
</body></html>`),
	"https://download.example/ServiceTags_Public.json": []byte(`{
		"values": [
			{"properties": {"region": "", "systemService": "", "addressPrefixes": ["13.64.0.0/11"]}},
			{"properties": {"region": "westus", "systemService": "AzureStorage", "addressPrefixes": ["20.150.0.0/24", "2603:1030::/45"]}}
		]
	}`),
	"https://www.cloudflare.com/ips-v4": []byte("173.245.48.0/20\n103.21.244.0/22\n"),
	"https://www.cloudflare.com/ips-v6": []byte("2400:cb00::/32\n"),
	"https://api.fastly.com/public-ip-list": []byte(`{
		"addresses": ["151.101.0.0/16", "23.235.32.0/20"],
		"ipv6_addresses": ["2a04:4e40::/32"]
	}`),
	"https://www.gstatic.com/ipranges/cloud.json": []byte(`{
		"prefixes": [
			{"ipv4Prefix": "34.1.208.0/20", "service": "Google Cloud", "scope": "africa-south1"},
			{"ipv6Prefix": "2600:1900:8000::/44", "service": "Google Cloud", "scope": "us-east4"}
		]
	}`),
	"https://docs.oracle.com/en-us/iaas/tools/public_ip_ranges.json": []byte(`{
		"regions": [
			{"region": "us-ashburn-1", "cidrs": [{"cidr": "129.213.0.0/16", "tags": ["OSN", "OCI"]}]}
		]
	}`),
	"https://www.spamhaus.org/drop/drop.lasso":  []byte("; Spamhaus DROP List\n1.10.16.0/20 ; SBL256894\n"),
	"https://www.spamhaus.org/drop/edrop.lasso": []byte("; Spamhaus EDROP List\n"),
	"https://www.spamhaus.org/drop/dropv6.txt":  []byte("; Spamhaus DROPv6 List\n2001:678:738::/48 ; SBL636050\n"),
}

var providerFlags = []string{"aws", "azure", "cloudflare", "fastly", "google-cloud", "oracle", "provider", "spamhaus"}

// Each feed's blocks in address order, IPv4 before IPv6.
var expectedFeedMaps = map[string][]string{
	"ip_to_aws": {
		"3.5.140.0/22 AWS/S3/ap-northeast-2",
		"15.230.56.104/31 AWS/AMAZON/us-east-1",
		"2600:1f14::/35 AWS/EC2/us-west-2",
	},
	"ip_to_azure": {
		"20.150.0.0/24 Azure/AzureStorage/westus",
		"2603:1030::/45 Azure/AzureStorage/westus",
	},
	"ip_to_cloudflare": {
		"103.21.244.0/22 Cloudflare",
		"173.245.48.0/20 Cloudflare",
		"2400:cb00::/32 Cloudflare",
	},
	"ip_to_fastly": {
		"23.235.32.0/20 Fastly",
		"151.101.0.0/16 Fastly",
		"2a04:4e40::/32 Fastly",
	},
	"ip_to_google_cloud": {
		"34.1.208.0/20 GC/Google Cloud/africa-south1",
		"2600:1900:8000::/44 GC/Google Cloud/us-east4",
	},
	"ip_to_oracle": {
		"129.213.0.0/16 Oracle/OCI,OSN/us-ashburn-1",
	},
	"ip_to_provider": {
		"3.5.140.0/22 AWS/S3/ap-northeast-2",
		"15.230.56.104/31 AWS/AMAZON/us-east-1",
		"20.150.0.0/24 Azure/AzureStorage/westus",
		"23.235.32.0/20 Fastly",
		"34.1.208.0/20 GC/Google Cloud/africa-south1",
		"103.21.244.0/22 Cloudflare",
		"129.213.0.0/16 Oracle/OCI,OSN/us-ashburn-1",
		"151.101.0.0/16 Fastly",
		"173.245.48.0/20 Cloudflare",
		"2400:cb00::/32 Cloudflare",
		"2600:1900:8000::/44 GC/Google Cloud/us-east4",
		"2600:1f14::/35 AWS/EC2/us-west-2",
		"2603:1030::/45 Azure/AzureStorage/westus",
		"2a04:4e40::/32 Fastly",
	},
	"ip_to_spamhaus": {
		"1.10.16.0/20 Spamhaus/SBL256894",
		"2001:678:738::/48 Spamhaus/SBL636050",
	},
}

func selecting(flags ...string) map[string]bool {
	maps := make(map[string]bool)
	for _, flag := range flags {
		maps[flag] = true
	}
	return maps
}

func ipv4Lines(lines []string) []string {
	v4 := make([]string, 0)
	for _, line := range lines {
		if !strings.Contains(strings.Fields(line)[0], ":") {
			v4 = append(v4, line)
		}
	}
	return v4
}

func TestFeedMaps(t *testing.T) {
	for _, ipv6 := range []bool{true, false} {
		outdir := t.TempDir()
		runPipeline(t, Options{
			Outdir:  outdir,
			Maps:    selecting(providerFlags...),
			IPv6:    ipv6,
			Fetcher: feedFixtures,
		})

		for name, expected := range expectedFeedMaps {
			if !ipv6 {
				expected = ipv4Lines(expected)
			}
			assertLines(t, name, readLines(t, filepath.Join(outdir, name)), expected)
		}
		if got := len(listDir(t, outdir)); got != len(expectedFeedMaps) {
			t.Errorf("ipv6=%v: %d files written, expected %d", ipv6, got, len(expectedFeedMaps))
		}
	}
}

func TestFeedMapHeaders(t *testing.T) {
	outdir := t.TempDir()
	runPipeline(t, Options{Outdir: outdir, Maps: selecting("aws", "fastly", "provider"), IPv6: true, Fetcher: feedFixtures})

	aws, err := os.ReadFile(filepath.Join(outdir, "ip_to_aws"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(aws), "#\n# IP to AWS mapping\n") || !strings.Contains(string(aws), "# cidr AWS/service/region\n") {
		t.Errorf("ip_to_aws header:\n%s", aws)
	}

	provider, err := os.ReadFile(filepath.Join(outdir, "ip_to_provider"))
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"# Merges\n#   aws\n#   fastly\n", "# cidr AWS/service/region\n# cidr Fastly\n"} {
		if !strings.Contains(string(provider), expected) {
			t.Errorf("ip_to_provider header lacks %q:\n%s", expected, provider)
		}
	}
}

func TestFeedErrors(t *testing.T) {
	for _, test := range []struct {
		desc     string
		flag     string
		url      string
		body     string
		expected string
	}{
		{"unreachable", "fastly", "https://api.fastly.com/public-ip-list", "", "fastly: no fixture for"},
		{"bad json", "aws", "https://ip-ranges.amazonaws.com/ip-ranges.json", "{", "aws: https://ip-ranges.amazonaws.com/ip-ranges.json: unexpected end of JSON input"},
		{"bad cidr", "cloudflare", "https://www.cloudflare.com/ips-v4", "1.2.3.4/33\n", "cloudflare: invalid CIDR address: 1.2.3.4/33"},
		{"bad droplist", "spamhaus", "https://www.spamhaus.org/drop/drop.lasso", "1.2.3.0/24\n", `spamhaus: https://www.spamhaus.org/drop/drop.lasso: unexpected droplist line "1.2.3.0/24"`},
		{"azure page changed", "azure", azureDownloadPage, "<html></html>", "azure: Failed to find the download url"},
	} {
		fetcher := fixtures{}
		for url, body := range feedFixtures {
			fetcher[url] = body
		}
		if test.body == "" {
			delete(fetcher, test.url)
		} else {
			fetcher[test.url] = []byte(test.body)
		}

		p, err := BuildPipeline(Options{Outdir: t.TempDir(), Maps: selecting(test.flag), IPv6: true, Fetcher: fetcher})
		if err != nil {
			t.Fatal(err)
		}
		err = p.Run()
		if err == nil || !strings.Contains(err.Error(), test.expected) {
			t.Errorf("%s: got error %v, expected %q", test.desc, err, test.expected)
		}
	}
}

func cityRecord(continent, country, subdivision, city, postal string) mmdbtype.Map {
	record := mmdbtype.Map{
		"continent": mmdbtype.Map{"code": mmdbtype.String(continent)},
		"country":   mmdbtype.Map{"iso_code": mmdbtype.String(country)},
		// Unused by the maps; differing postal codes keep the database from
		// merging otherwise identical neighbouring networks.
		"postal": mmdbtype.Map{"code": mmdbtype.String(postal)},
	}
	if subdivision != "" {
		record["subdivisions"] = mmdbtype.Slice{mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(subdivision)}}}
	}
	if city != "" {
		record["city"] = mmdbtype.Map{"names": mmdbtype.Map{"en": mmdbtype.String(city)}}
	}
	return record
}

var cityRecords = []mmdbRecord{
	{"1.0.0.0/25", cityRecord("OC", "AU", "Queensland", "Brisbane", "4000")},
	{"1.0.0.128/25", cityRecord("OC", "AU", "Queensland", "Brisbane", "4001")},
	{"1.0.1.0/24", cityRecord("AS", "CN", "", "", "")},
	{"2001:db8::1/128", cityRecord("AS", "CN", "", "", "")},
}

var expectedCityMaps = map[string][]string{
	"ip_to_continent":    {"1.0.0.0/24 OC", "1.0.1.0/24 AS", "2001:db8::1/128 AS"},
	"ip_to_country":      {"1.0.0.0/24 AU", "1.0.1.0/24 CN", "2001:db8::1/128 CN"},
	"ip_to_location":     {"1.0.0.0/24 OC/AU/Queensland/Brisbane", "1.0.1.0/24 AS/CN", "2001:db8::1/128 AS/CN"},
	"ip_to_subdivisions": {"1.0.0.0/24 Queensland"},
	"ip_to_city":         {"1.0.0.0/24 Brisbane"},
}

var asnRecords = []mmdbRecord{
	{"1.1.1.0/25", mmdbtype.Map{"autonomous_system_number": mmdbtype.Uint32(13335), "autonomous_system_organization": mmdbtype.String("CLOUDFLARENET")}},
	{"1.1.1.128/25", mmdbtype.Map{"autonomous_system_number": mmdbtype.Uint32(13335), "autonomous_system_organization": mmdbtype.String("Cloudflare")}},
	{"8.8.8.0/24", mmdbtype.Map{"autonomous_system_number": mmdbtype.Uint32(15169), "autonomous_system_organization": mmdbtype.String("GOOGLE")}},
}

var ispRecords = []mmdbRecord{
	{"1.1.1.0/24", mmdbtype.Map{"autonomous_system_number": mmdbtype.Uint32(13335), "isp": mmdbtype.String("Cloudflare")}},
	{"8.8.8.0/24", mmdbtype.Map{"autonomous_system_number": mmdbtype.Uint32(15169), "isp": mmdbtype.String("Google")}},
}

func TestMaxMindMaps(t *testing.T) {
	dbs := t.TempDir()
	cityDb := writeFile(t, filepath.Join(dbs, "city.mmdb"), buildMMDB(t, "GeoIP2-City", cityRecords))
	countryDb := writeFile(t, filepath.Join(dbs, "country.mmdb"), buildMMDB(t, "GeoLite2-Country", cityRecords))
	asnDb := writeFile(t, filepath.Join(dbs, "asn.mmdb"), buildMMDB(t, "GeoLite2-ASN", asnRecords))
	ispDb := writeFile(t, filepath.Join(dbs, "isp.mmdb"), buildMMDB(t, "GeoIP2-ISP", ispRecords))

	cityFlags := []string{"continent", "country", "location", "subdivisions", "city"}

	t.Run("city", func(t *testing.T) {
		for _, db := range []string{cityDb, countryDb} {
			for _, ipv6 := range []bool{true, false} {
				outdir := t.TempDir()
				runPipeline(t, Options{Outdir: outdir, Maps: selecting(cityFlags...), CityDb: db, IPv6: ipv6})
				for name, expected := range expectedCityMaps {
					if !ipv6 {
						expected = ipv4Lines(expected)
					}
					assertLines(t, name, readLines(t, filepath.Join(outdir, name)), expected)
				}
			}
		}
	})

	t.Run("asn", func(t *testing.T) {
		outdir := t.TempDir()
		runPipeline(t, Options{Outdir: outdir, Maps: selecting("asn"), AsnDb: asnDb, IPv6: true})
		assertLines(t, "ip_to_asn", readLines(t, filepath.Join(outdir, "ip_to_asn")), []string{"1.1.1.0/24 AS13335", "8.8.8.0/24 AS15169"})
	})

	t.Run("isp", func(t *testing.T) {
		outdir := t.TempDir()
		runPipeline(t, Options{Outdir: outdir, Maps: selecting("asn", "isp"), IspDb: ispDb, IPv6: true})
		assertLines(t, "ip_to_asn", readLines(t, filepath.Join(outdir, "ip_to_asn")), []string{"1.1.1.0/24 AS13335", "8.8.8.0/24 AS15169"})
		assertLines(t, "ip_to_isp", readLines(t, filepath.Join(outdir, "ip_to_isp")), []string{"1.1.1.0/24 Cloudflare", "8.8.8.0/24 Google"})
	})

	t.Run("all includes isp only with ISP data", func(t *testing.T) {
		for _, test := range []struct {
			asnDb, ispDb string
			isp          bool
		}{
			{asnDb: asnDb},
			{ispDb: ispDb, isp: true},
		} {
			outdir := t.TempDir()
			runPipeline(t, Options{Outdir: outdir, All: true, AsnDb: test.asnDb, IspDb: test.ispDb, CityDb: cityDb, IPv6: true, Fetcher: feedFixtures})
			files := len(expectedFeedMaps) + len(expectedCityMaps) + 1
			if test.isp {
				files++
			}
			if got := len(listDir(t, outdir)); got != files {
				t.Errorf("isp=%v: %d files written, expected %d: %v", test.isp, got, files, listDir(t, outdir))
			}
		}
	})

	t.Run("isp-db without ISP data", func(t *testing.T) {
		_, err := BuildPipeline(Options{Outdir: t.TempDir(), Maps: selecting("isp"), IspDb: asnDb, IPv6: true})
		if err == nil || !strings.Contains(err.Error(), "does not include ISP data") {
			t.Errorf("got error %v", err)
		}
	})
}

func TestCountryDir(t *testing.T) {
	countryDb := writeFile(t, filepath.Join(t.TempDir(), "country.mmdb"), buildMMDB(t, "GeoLite2-Country", cityRecords))

	countryDir := filepath.Join(t.TempDir(), "countries")
	if err := os.MkdirAll(countryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(countryDir, "XX.map"), []byte("10.0.0.0/8\n"))
	writeFile(t, filepath.Join(countryDir, "README"), []byte("kept\n"))

	outdir := t.TempDir()
	runPipeline(t, Options{
		Outdir:    outdir,
		Maps:      selecting("country"),
		CityDb:    countryDb,
		SplitDirs: map[string]string{"country": countryDir},
		IPv6:      true,
	})

	assertLines(t, "files", listDir(t, countryDir), []string{"AU.map", "CN.map", "README"})
	assertLines(t, "AU.map", readLines(t, filepath.Join(countryDir, "AU.map")), []string{"1.0.0.0/24"})
	assertLines(t, "CN.map", readLines(t, filepath.Join(countryDir, "CN.map")), []string{"1.0.1.0/24", "2001:db8::1/128"})
	assertLines(t, "ip_to_country", readLines(t, filepath.Join(outdir, "ip_to_country")), expectedCityMaps["ip_to_country"])

	// Only the country source is needed, no Outdir.
	countryDir = t.TempDir()
	runPipeline(t, Options{CityDb: countryDb, SplitDirs: map[string]string{"country": countryDir}, IPv6: true})
	assertLines(t, "files without -country", listDir(t, countryDir), []string{"AU.map", "CN.map"})
}

func TestGeoLite2(t *testing.T) {
	const key = "test-key"
	downloads := fixtures{
		geoLite2URLFor("GeoLite2-ASN", key):     geoLite2Archive(t, "GeoLite2-ASN", buildMMDB(t, "GeoLite2-ASN", asnRecords)),
		geoLite2URLFor("GeoLite2-City", key):    geoLite2Archive(t, "GeoLite2-City", buildMMDB(t, "GeoLite2-City", cityRecords)),
		geoLite2URLFor("GeoLite2-Country", key): geoLite2Archive(t, "GeoLite2-Country", buildMMDB(t, "GeoLite2-Country", cityRecords)),
	}
	// only serves the named editions, so a run fails if it downloads others
	only := func(editions ...string) fixtures {
		f := fixtures{}
		for _, edition := range editions {
			f[geoLite2URLFor(edition, key)] = downloads[geoLite2URLFor(edition, key)]
		}
		return f
	}

	t.Run("country dir downloads Country", func(t *testing.T) {
		countryDir := t.TempDir()
		runPipeline(t, Options{GeoLite2: true, LicenseKey: key, SplitDirs: map[string]string{"country": countryDir}, IPv6: true,
			Fetcher: only("GeoLite2-Country")})
		assertLines(t, "files", listDir(t, countryDir), []string{"AU.map", "CN.map"})
		assertLines(t, "CN.map", readLines(t, filepath.Join(countryDir, "CN.map")), []string{"1.0.1.0/24", "2001:db8::1/128"})
	})

	t.Run("city maps download City and ASN", func(t *testing.T) {
		outdir := t.TempDir()
		runPipeline(t, Options{Outdir: outdir, Maps: selecting("asn", "continent", "location"), GeoLite2: true, LicenseKey: key, IPv6: true,
			Fetcher: only("GeoLite2-ASN", "GeoLite2-City")})
		assertLines(t, "ip_to_asn", readLines(t, filepath.Join(outdir, "ip_to_asn")), []string{"1.1.1.0/24 AS13335", "8.8.8.0/24 AS15169"})
		assertLines(t, "ip_to_location", readLines(t, filepath.Join(outdir, "ip_to_location")), expectedCityMaps["ip_to_location"])
	})

	t.Run("all", func(t *testing.T) {
		fetcher := only("GeoLite2-ASN", "GeoLite2-City")
		for url, body := range feedFixtures {
			fetcher[url] = body
		}
		outdir := t.TempDir()
		runPipeline(t, Options{Outdir: outdir, All: true, GeoLite2: true, LicenseKey: key, IPv6: true, Fetcher: fetcher})
		if got, expected := len(listDir(t, outdir)), len(expectedFeedMaps)+len(expectedCityMaps)+1; got != expected {
			t.Errorf("%d files written, expected %d", got, expected)
		}
	})

	t.Run("download failure hides the key", func(t *testing.T) {
		_, err := BuildPipeline(Options{Outdir: t.TempDir(), Maps: selecting("asn"), GeoLite2: true, LicenseKey: key, Fetcher: fixtures{}})
		if err == nil || !strings.Contains(err.Error(), "downloading GeoLite2-ASN: no fixture for") || strings.Contains(err.Error(), key) {
			t.Errorf("got error %v", err)
		}
	})

	t.Run("archive without the database", func(t *testing.T) {
		fetcher := fixtures{geoLite2URLFor("GeoLite2-ASN", key): geoLite2Archive(t, "GeoLite2-City", []byte("db"))}
		_, err := BuildPipeline(Options{Outdir: t.TempDir(), Maps: selecting("asn"), GeoLite2: true, LicenseKey: key, Fetcher: fetcher})
		if err == nil || !strings.Contains(err.Error(), "GeoLite2-ASN.mmdb not found in archive") {
			t.Errorf("got error %v", err)
		}
	})
}

func TestOptionErrors(t *testing.T) {
	for _, test := range []struct {
		desc     string
		opts     Options
		expected string
	}{
		{"nothing selected", Options{Outdir: "out"}, "no outputs specified"},
		{"no outdir", Options{Maps: selecting("aws")}, "missing required argument -outdir"},
		{"provider alone", Options{Outdir: "out", Maps: selecting("provider", "spamhaus")}, "argument -provider enabled, but none of -aws, -azure"},
		{"asn without a database", Options{Outdir: "out", Maps: selecting("asn")}, "neither -asn-db nor -isp-db specified"},
		{"isp with an ASN database", Options{Outdir: "out", Maps: selecting("isp"), AsnDb: "asn.mmdb"}, "argument -isp enabled, but -isp-db not specified"},
		{"both ASN and ISP databases", Options{Outdir: "out", Maps: selecting("asn"), AsnDb: "a", IspDb: "i"}, "-asn-db and -isp-db both specified"},
		{"isp database unused", Options{Outdir: "out", Maps: selecting("aws"), IspDb: "i"}, "argument -isp-db provided, but no associated maps"},
		{"asn database unused", Options{Outdir: "out", Maps: selecting("aws"), AsnDb: "a"}, "argument -asn-db provided, but no associated maps"},
		{"city database unused", Options{Outdir: "out", Maps: selecting("aws"), CityDb: "c"}, "argument -city-db provided, but no associated maps"},
		{"city map without a database", Options{Outdir: "out", Maps: selecting("city")}, "the city map needs -city-db or -geolite2"},
		{"country dir without a database", Options{SplitDirs: map[string]string{"country": "c"}}, "the country map needs -city-db or -geolite2"},
		{"all without databases", Options{Outdir: "out", All: true}, "neither -asn-db nor -isp-db specified"},
		{"geolite2 with a database", Options{Outdir: "out", Maps: selecting("city"), GeoLite2: true, LicenseKey: "k", CityDb: "c"}, "drop -asn-db, -isp-db and -city-db"},
		{"geolite2 without a key", Options{Outdir: "out", Maps: selecting("city"), GeoLite2: true}, "requires the MAXMIND_LICENSE_KEY"},
		{"geolite2 isp", Options{Outdir: "out", Maps: selecting("isp"), GeoLite2: true, LicenseKey: "k"}, "argument -isp enabled, but the free GeoLite2 databases do not include its data"},
		{"geolite2 unused", Options{Outdir: "out", Maps: selecting("aws"), GeoLite2: true, LicenseKey: "k"}, "argument -geolite2 provided, but no associated maps"},
	} {
		_, err := BuildPipeline(test.opts)
		if err == nil || !strings.Contains(err.Error(), test.expected) {
			t.Errorf("%s: got error %v, expected %q", test.desc, err, test.expected)
		}
	}
}
