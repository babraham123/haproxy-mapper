#!/usr/bin/env python3
# Generates one HAProxy map per country, <ISO code>.map with a CIDR per line, from the
# MaxMind GeoLite2 Country database. Built into an image with haproxy-mapper by
# geoip_generator.Containerfile.
# Environment:
#   MAXMIND_LICENSE_KEY  required, to download the database
#   OUTPUT_DIR           where the maps go, default /etc/haproxy/countries; its *.map files
#                        are replaced on every run

import os
import sys
import shutil
import urllib.request
import urllib.error
import tarfile
import tempfile
import subprocess
import re
import glob

haproxy_mapper_bin = "/usr/local/bin/haproxy-mapper"

def main():
  maxmind_key = os.environ.get("MAXMIND_LICENSE_KEY", "").strip()
  if not maxmind_key:
    print("Error: MAXMIND_LICENSE_KEY is not set.", file=sys.stderr)
    sys.exit(1)
  output_dir = os.environ.get("OUTPUT_DIR", "").strip() or "/etc/haproxy/countries"

  if not shutil.which(haproxy_mapper_bin):
    print(f"Error: Required command '{haproxy_mapper_bin}' not found.", file=sys.stderr)
    sys.exit(1)

  with tempfile.TemporaryDirectory() as tmp_dir:
    # Download Country Database
    db_url = f"https://download.maxmind.com/app/geoip_download?edition_id=GeoLite2-Country&license_key={maxmind_key}&suffix=tar.gz"
    tar_path = os.path.join(tmp_dir, "GeoLite2-Country.tar.gz")

    try:
      urllib.request.urlretrieve(db_url, tar_path)
    except urllib.error.URLError as e:
      print(f"Error: Download failed. ({e})", file=sys.stderr)
      sys.exit(1)

    # Extract Database
    with tarfile.open(tar_path, "r:gz") as tar:
      tar.extractall(path=tmp_dir)

    # Look for Country .mmdb
    country_db = None
    for root, _, files in os.walk(tmp_dir):
      for file in files:
        if file == "GeoLite2-Country.mmdb":
          country_db = os.path.join(root, file)
          break

    if not country_db:
      print("Error: GeoLite2-Country.mmdb not found in archive.", file=sys.stderr)
      sys.exit(1)

    # Generate Maps
    mapper_out = os.path.join(tmp_dir, "mapper_output")
    os.makedirs(mapper_out, exist_ok=True)
    try:
      # The Country database shares the City schema, so -city-db reads it.
      subprocess.run(
        [haproxy_mapper_bin, "-outdir", mapper_out, "-city-db", country_db, "-country"],
        check=True,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL
      )
    except subprocess.CalledProcessError:
      print("Error: haproxy-mapper execution failed.", file=sys.stderr)
      sys.exit(1)

    # Split ip_to_country into one map per country
    ip_to_country_file = os.path.join(mapper_out, "ip_to_country")
    os.makedirs(output_dir, exist_ok=True)
    for old_map in glob.glob(os.path.join(output_dir, "*.map")):
      os.remove(old_map)

    country_pattern = re.compile(r"^[A-Za-z0-9]+$")
    file_handles = {}
    try:
      with open(ip_to_country_file, "r") as f:
        for line in f:
          parts = line.strip().split()
          if len(parts) >= 2:
            cidr, country = parts[0], parts[1]
            if country_pattern.match(country):
              if country not in file_handles:
                file_handles[country] = open(os.path.join(output_dir, f"{country}.map"), "w")
              file_handles[country].write(f"{cidr}\n")
    finally:
      for fh in file_handles.values():
        fh.close()

    print(f"Successfully Generated {len(file_handles)} country files in: {output_dir}")

if __name__ == "__main__":
  main()
