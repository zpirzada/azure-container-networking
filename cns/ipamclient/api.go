package ipamclient

// IPAM Plugin API Contract.
const (
	getAddressSpacesPath = "/IpamDriver.GetDefaultAddressSpaces"
	requestPoolPath      = "/IpamDriver.RequestPool"
	reserveAddrPath      = "/IpamDriver.RequestAddress"
	releaseAddrPath      = "/IpamDriver.ReleaseAddress"
	getPoolInfoPath      = "/IpamDriver.GetPoolInfo"
)

// Response received from IPAM Plugin when request AddressSpace.
type getAddressSpacesResponse struct {
	Err                       string
	LocalDefaultAddressSpace  string
	GlobalDefaultAddressSpace string
}

// Request sent to IPAM plugin to request a pool.
type requestPoolRequest struct {
	AddressSpace string
	Pool         string
	SubPool      string
	Options      map[string]string
	V6           bool
}

// Response received from IPAM Plugin when requesting a pool.
type requestPoolResponse struct {
	Err    string
	PoolID string
	Pool   string
	Data   map[string]string
}

// Request sent to IPAM plugin to request IP reservation.
type reserveAddrRequest struct {
	PoolID  string
	Address string
	Options map[string]string
}

// Response received from IPAM Plugin when requesting a IP reservation.
type reserveAddrResponse struct {
	Err     string
	Address string
	Data    map[string]string
}

// Request sent to IPAM plugin to release IP reservation.
type releaseAddrRequest struct {
	PoolID  string
	Address string
	Options map[string]string
}

// Response received from IPAM Plugin when requesting IP release.
type releaseAddrResponse struct {
	Err string
}

// Request sent to IPAM plugin to query address pool information.
type getPoolInfoRequest struct {
	PoolID string
}

// Response sent by plugin when returning address pool information.
type getPoolInfoResponse struct {
	Err                string
	Capacity           int
	Available          int
	UnhealthyAddresses []string
}
