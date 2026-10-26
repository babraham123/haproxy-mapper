package main

import (
	"fmt"
	"net"
	"net/netip"
)

// CombiningProcessor merges runs of adjacent blocks with the same value into
// the fewest CIDRs that cover them. It expects blocks in address order, which
// is how the MaxMind origins emit them.
type CombiningProcessor struct {
	// The run being built: first..last all share value. open is false until
	// the first block arrives.
	first netip.Addr
	last  netip.Addr
	value string
	open  bool
	Emitter
}

func CombiningProcessorCreate() *CombiningProcessor {
	return &CombiningProcessor{}
}

func (c *CombiningProcessor) Subscribed(id string) {
}

func (c *CombiningProcessor) Header(id string, header Header) error {
	header.general += "# Adjacent blocks with matching values have been combined\n#\n"
	return c.Emitter.Header(header)
}

func (c *CombiningProcessor) Receive(id string, block *Block) error {
	prefix, ok := prefixFromIPNet(block.net)
	if !ok {
		return fmt.Errorf("invalid network %v", block.net)
	}
	first := prefix.Addr()
	last := lastAddr(prefix)

	// Next() of the highest address is the zero Addr, which never matches, and
	// v4 and v6 addresses never compare equal, so runs can't wrap or cross
	// families.
	if c.open && c.value == *block.value && c.last.Next() == first {
		c.last = last
		return nil
	}

	if err := c.flush(); err != nil {
		return err
	}
	c.first = first
	c.last = last
	c.value = *block.value
	c.open = true
	return nil
}

func (c *CombiningProcessor) Done(id string) error {
	if err := c.flush(); err != nil {
		return err
	}
	return c.Emitter.Done()
}

func (c *CombiningProcessor) flush() error {
	if !c.open {
		return nil
	}
	c.open = false

	value := c.value
	for _, prefix := range rangePrefixes(c.first, c.last) {
		ipNet := &net.IPNet{
			IP:   prefix.Addr().AsSlice(),
			Mask: net.CIDRMask(prefix.Bits(), prefix.Addr().BitLen()),
		}
		if err := c.Emit(BlockCreate(ipNet, &value)); err != nil {
			return err
		}
	}
	return nil
}

func prefixFromIPNet(n *net.IPNet) (netip.Prefix, bool) {
	addr, ok := netip.AddrFromSlice(n.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ones, bits := n.Mask.Size()
	if addr.Is4In6() && bits == 32 {
		addr = addr.Unmap()
	}
	if bits != addr.BitLen() {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(addr, ones).Masked(), true
}

// lastAddr is the highest address within prefix.
func lastAddr(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Masked().Addr().AsSlice()
	for i := prefix.Bits(); i < len(bytes)*8; i++ {
		bytes[i/8] |= 0x80 >> (i % 8)
	}
	addr, _ := netip.AddrFromSlice(bytes)
	return addr
}

// rangePrefixes is the smallest list of prefixes covering first..last, in
// address order.
func rangePrefixes(first, last netip.Addr) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0)
	for {
		// The largest prefix that starts at first and ends at or before last.
		bits := 0
		for ; bits < first.BitLen(); bits++ {
			candidate := netip.PrefixFrom(first, bits)
			if candidate.Masked().Addr() == first && lastAddr(candidate).Compare(last) <= 0 {
				break
			}
		}
		prefix := netip.PrefixFrom(first, bits)
		prefixes = append(prefixes, prefix)

		end := lastAddr(prefix)
		if end.Compare(last) >= 0 {
			return prefixes
		}
		first = end.Next()
	}
}
