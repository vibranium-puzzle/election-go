package election

type ScriptCommand struct {
	Name string
	Args []string
}

type FloatIpConfig struct {
	Dev         string
	VirtualDev  string
	IpVersion   IpVersion
	VirtualIp   string
	NetworkMask string
	Gateway     string

	CommandIpUp   *ScriptCommand
	CommandIpDown *ScriptCommand
	CommandArp    *ScriptCommand
}

func (f *FloatIpConfig) UpCmd() *ScriptCommand {
	if f.CommandIpUp != nil {
		return f.CommandIpUp
	}
	return &ScriptCommand{
		Name: "/sbin/ifconfig",
		Args: []string{f.VirtualDev, f.VirtualIp, "netmask", f.NetworkMask, "up"},
	}
}

func (f *FloatIpConfig) DownCmd() *ScriptCommand {
	if f.CommandIpDown != nil {
		return f.CommandIpDown
	}
	return &ScriptCommand{
		Name: "/sbin/ifconfig",
		Args: []string{f.VirtualDev, f.VirtualIp, "netmask", f.NetworkMask, "down"},
	}
}

func (f *FloatIpConfig) ArpCmd() *ScriptCommand {
	if f.CommandArp != nil {
		return f.CommandArp
	}
	return &ScriptCommand{
		Name: "/sbin/arping",
		Args: []string{"-I", f.Dev, "-w", "0", "-c", "1", "-U", "-s", f.VirtualIp, f.VirtualIp},
	}
}
