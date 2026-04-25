package skills

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const toolHuaweiSwitch = "builtin_huawei_switch"

var allowedHuaweiOps = map[string]bool{
	"display_vlan":                     true,
	"display_interface":                true,
	"display_version":                  true,
	"display_ip_route":                 true,
	"display_ip_route_verbose":         true,
	"display_arp":                      true,
	"display_mac_address":              true,
	"display_stp":                      true,
	"display_power":                    true,
	"display_fan":                      true,
	"display_temperature":              true,
	"display_cpu":                      true,
	"display_memory":                   true,
	"display_device":                   true,
	"display_ospf_neighbor":            true,
	"display_ospf_interface":           true,
	"display_ospf_lsdb":                true,
	"display_bgp_peer":                 true,
	"display_bgp_routing_table":        true,
	"display_rip_neighbor":             true,
	"display_rip_route":                true,
	"display_fib":                      true,
	"display_routing_table_statistics": true,
	"display_bfd_session":              true,
	"display_clock":                    true,
	"display_license":                  true,
	"display_poe_status":               true,
	"display_qos":                      true,
	"display_nat_session":              true,
	"display_nat_server":               true,
	"display_acl_all":                  true,
}

var allowedHuaweiCmds = map[string]string{
	"display_vlan":                     "display vlan",
	"display_interface":                "display interface brief",
	"display_version":                  "display version",
	"display_ip_route":                 "display ip routing-table",
	"display_ip_route_verbose":         "display ip routing-table verbose",
	"display_arp":                      "display arp",
	"display_mac_address":              "display mac-address",
	"display_stp":                      "display stp brief",
	"display_power":                    "display power",
	"display_fan":                      "display fan",
	"display_temperature":              "display temperature all",
	"display_cpu":                      "display cpu-usage",
	"display_memory":                   "display memory",
	"display_device":                   "display device",
	"display_ospf_neighbor":            "display ospf peer brief",
	"display_ospf_interface":           "display ospf interface",
	"display_ospf_lsdb":                "display ospf lsdb",
	"display_bgp_peer":                 "display bgp peer",
	"display_bgp_routing_table":        "display bgp routing-table",
	"display_rip_neighbor":             "display rip neighbor",
	"display_rip_route":                "display rip route",
	"display_fib":                      "display fib",
	"display_routing_table_statistics": "display routing-table statistics",
	"display_bfd_session":              "display bfd session all",
	"display_clock":                    "display clock",
	"display_license":                  "display license",
	"display_poe_status":               "display poe interface",
	"display_qos":                      "display qos policy brief",
	"display_nat_session":              "display nat session",
	"display_nat_server":               "display nat server",
	"display_acl_all":                  "display acl all",
}

func execBuiltinHuaweiSwitch(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		return "", fmt.Errorf("missing operation")
	}

	if !allowedHuaweiOps[op] {
		return "", fmt.Errorf("operation %q not allowed (allowed: %v)", op, allowedHuaweiOps)
	}

	host := strArg(in, "host", "address", "ip")
	if host == "" {
		return "", fmt.Errorf("missing host")
	}

	port := strArg(in, "port", "ssh_port")
	if port == "" {
		port = "22"
	}

	user := strArg(in, "user", "username")
	if user == "" {
		user = "admin"
	}

	password := strArg(in, "password", "pass")
	cmdStr := strArg(in, "command", "cmd")
	if cmdStr == "" {
		cmdStr = allowedHuaweiCmds[op]
	}

	sshCmd := fmt.Sprintf("ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -p %s %s@%s '%s'", port, user, host, cmdStr)

	if password != "" {
		sshCmd = fmt.Sprintf("sshpass -p %s ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 -p %s %s@%s '%s'", password, port, user, host, cmdStr)
	}

	cmd := exec.Command("sh", "-c", sshCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("SSH to %s failed: %s\n%s", host, err.Error(), string(output))
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return fmt.Sprintf("Huawei switch %s: no output from command", host), nil
	}

	return fmt.Sprintf("Huawei switch %s [%s]:\n\n%s", host, op, result), nil
}

func NewBuiltinHuaweiSwitchTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolHuaweiSwitch,
			Desc: "Huawei network device (switch/router) operations: display vlan, interface, version, ip route, ospf, bgp, rip, bfd, nat, acl, stp, power, fan, temperature, cpu, memory. Uses SSH.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: display_vlan, display_interface, display_version, display_ip_route, display_ip_route_verbose, display_arp, display_mac_address, display_stp, display_power, display_fan, display_temperature, display_cpu, display_memory, display_device, display_ospf_neighbor, display_ospf_interface, display_ospf_lsdb, display_bgp_peer, display_bgp_routing_table, display_rip_neighbor, display_rip_route, display_fib, display_routing_table_statistics, display_bfd_session, display_clock, display_license, display_poe_status, display_qos, display_nat_session, display_nat_server, display_acl_all", Required: true},
				"host":      {Type: einoschema.String, Desc: "Device IP address or hostname", Required: true},
				"port":      {Type: einoschema.String, Desc: "SSH port (default: 22)", Required: false},
				"user":      {Type: einoschema.String, Desc: "SSH username (default: admin)", Required: false},
				"password":  {Type: einoschema.String, Desc: "SSH password", Required: false},
				"command":   {Type: einoschema.String, Desc: "Custom display command (optional)", Required: false},
			}),
		},
		execBuiltinHuaweiSwitch,
	)
}
