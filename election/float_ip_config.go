package election

type FloatIpConfig struct {
	Dev         string
	VirtualDev  string
	IpVersion   IpVersion
	VirtualIp   string
	NetworkMask string
	Gateway     string
}

func (f *FloatIpConfig) UpCmd() string {
	return "/sbin/ifconfig " + f.VirtualDev + " " + f.VirtualIp + " netmask " + f.NetworkMask + " up"
}

func (f *FloatIpConfig) DownCmd() string {
	return "/sbin/ifconfig " + f.VirtualDev + " " + f.VirtualIp + " netmask " + f.NetworkMask + " down"
}

func (f *FloatIpConfig) ArpCmd() string {
	return "/sbin/arping -I " + f.Dev + " -w 0 -c 1 -U -s " + f.VirtualIp + " " + f.VirtualIp
}
