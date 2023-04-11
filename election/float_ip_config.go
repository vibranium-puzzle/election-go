package election

type FloatIpConfig struct {
	Dev         string
	VirtualDev  string
	IpVersion   IpVersion
	VirtualIp   string
	NetworkMask string
	Gateway     string

	CommandIpUp   string
	CommandIpDown string
	CommandArp    string
}

func (f *FloatIpConfig) UpCmd() string {
	if f.CommandIpUp != "" {
		return f.CommandIpUp
	}
	return "/sbin/ifconfig " + f.VirtualDev + " " + f.VirtualIp + " netmask " + f.NetworkMask + " up"
}

func (f *FloatIpConfig) DownCmd() string {
	if f.CommandIpDown != "" {
		return f.CommandIpDown
	}
	return "/sbin/ifconfig " + f.VirtualDev + " " + f.VirtualIp + " netmask " + f.NetworkMask + " down"
}

func (f *FloatIpConfig) ArpCmd() string {
	if f.CommandArp != "" {
		return f.CommandArp
	}
	return "/sbin/arping -I " + f.Dev + " -w 0 -c 1 -U -s " + f.VirtualIp + " " + f.VirtualIp
}
