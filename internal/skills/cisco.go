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

const toolCiscoIOS = "builtin_cisco_ios"

var allowedCiscoOps = map[string]bool{
	"show_vlan":                 true,
	"show_interface":            true,
	"show_interface_status":     true,
	"show_interface_trunk":      true,
	"show_version":              true,
	"show_ip_route":             true,
	"show_ip_route_detailed":    true,
	"show_arp":                  true,
	"show_mac_address":          true,
	"show_spanning_tree":        true,
	"show_spanning_tree_detail": true,
	"show_power":                true,
	"show_environment":          true,
	"show_cpu":                  true,
	"show_memory":               true,
	"show_inventory":            true,
	"show_ospf":                 true,
	"show_ospf_neighbor":        true,
	"show_ospf_interface":       true,
	"show_ospf_database":        true,
	"show_bgp":                  true,
	"show_bgp_neighbor":         true,
	"show_bgp_routing_table":    true,
	"show_eigrp":                true,
	"show_eigrp_neighbor":       true,
	"show_eigrp_top":            true,
	"show_ip_rip_database":      true,
	"show_clock":                true,
	"show_license":              true,
	"show_vtp_status":           true,
	"show_port_channel":         true,
	"show_port_channel_summary": true,
	"show_cdp_neighbors":        true,
	"show_cdp_neighbors_detail": true,
	"show_nat_translation":      true,
	"show_nat_statistics":       true,
	"show_access_lists":         true,
	"show_ip_cef":               true,
	"show_ip_cef_detail":        true,
	"show_failover":             true,
}

var allowedCiscoCmds = map[string]string{
	"show_vlan":                 "show vlan",
	"show_interface":            "show interfaces",
	"show_interface_status":     "show interface status",
	"show_interface_trunk":      "show interface trunk",
	"show_version":              "show version",
	"show_ip_route":             "show ip route",
	"show_ip_route_detailed":    "show ip route detail",
	"show_arp":                  "show arp",
	"show_mac_address":          "show mac address-table",
	"show_spanning_tree":        "show spanning-tree brief",
	"show_spanning_tree_detail": "show spanning-tree detail",
	"show_power":                "show power",
	"show_environment":          "show environment all",
	"show_cpu":                  "show cpu",
	"show_memory":               "show memory",
	"show_inventory":            "show inventory",
	"show_ospf":                 "show ip ospf",
	"show_ospf_neighbor":        "show ip ospf neighbor",
	"show_ospf_interface":       "show ip ospf interface",
	"show_ospf_database":        "show ip ospf database",
	"show_bgp":                  "show ip bgp",
	"show_bgp_neighbor":         "show ip bgp neighbors",
	"show_bgp_routing_table":    "show ip bgp routing-table",
	"show_eigrp":                "show ip eigrp protocols",
	"show_eigrp_neighbor":       "show ip eigrp neighbors",
	"show_eigrp_top":            "show ip eigrp topology",
	"show_ip_rip_database":      "show ip rip database",
	"show_clock":                "show clock",
	"show_license":              "show license",
	"show_vtp_status":           "show vtp status",
	"show_port_channel":         "show port-channel summary",
	"show_port_channel_summary": "show port-channel summary",
	"show_cdp_neighbors":        "show cdp neighbors",
	"show_cdp_neighbors_detail": "show cdp neighbors detail",
	"show_nat_translation":      "show ip nat translations",
	"show_nat_statistics":       "show ip nat statistics",
	"show_access_lists":         "show access-lists",
	"show_ip_cef":               "show ip cef",
	"show_ip_cef_detail":        "show ip cef detail",
	"show_failover":             "show failover",
}

func execBuiltinCiscoIOS(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		return "", fmt.Errorf("missing operation")
	}

	if !allowedCiscoOps[op] {
		return "", fmt.Errorf("operation %q not allowed (allowed: %v)", op, allowedCiscoOps)
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
		cmdStr = allowedCiscoCmds[op]
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
		return fmt.Sprintf("Cisco IOS %s: no output from command", host), nil
	}

	return fmt.Sprintf("Cisco IOS %s [%s]:\n\n%s", host, op, result), nil
}

func NewBuiltinCiscoIOSTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name: toolCiscoIOS,
			Desc: "Cisco IOS network device (switch/router) operations: show vlan, interface, version, ip route, OSPF, BGP, EIGRP, RIP, NAT, ACL, CEF, spanning-tree, power, environment, cpu, memory, inventory, CDP, port-channel, failover. Uses SSH.",
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation": {Type: einoschema.String, Desc: "Operation: show_vlan, show_interface, show_interface_status, show_interface_trunk, show_version, show_ip_route, show_ip_route_detailed, show_arp, show_mac_address, show_spanning_tree, show_spanning_tree_detail, show_power, show_environment, show_cpu, show_memory, show_inventory, show_ospf, show_ospf_neighbor, show_ospf_interface, show_ospf_database, show_bgp, show_bgp_neighbor, show_bgp_routing_table, show_eigrp, show_eigrp_neighbor, show_eigrp_top, show_ip_rip_database, show_clock, show_license, show_vtp_status, show_port_channel, show_port_channel_summary, show_cdp_neighbors, show_cdp_neighbors_detail, show_nat_translation, show_nat_statistics, show_access_lists, show_ip_cef, show_ip_cef_detail, show_failover", Required: true},
				"host":      {Type: einoschema.String, Desc: "Switch IP address or hostname", Required: true},
				"port":      {Type: einoschema.String, Desc: "SSH port (default: 22)", Required: false},
				"user":      {Type: einoschema.String, Desc: "SSH username (default: admin)", Required: false},
				"password":  {Type: einoschema.String, Desc: "SSH password", Required: false},
				"command":   {Type: einoschema.String, Desc: "Custom show command (optional)", Required: false},
			}),
		},
		execBuiltinCiscoIOS,
	)
}
