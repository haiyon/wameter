## IP Address Change Detected

**{{.Action | toTitle}} - {{.Reason | toTitle}}**

{{if .IsExternal}}
### External IP Change
> Agent ID: {{.Agent.ID}}
> Hostname: {{.Agent.Hostname}}
> IP Version: {{.Version}}
{{if .OldAddrs}}> Old IP: {{index .OldAddrs 0}}{{end}}
{{if .NewAddrs}}> New IP: {{index .NewAddrs 0}}{{end}}
{{else}}
### Interface IP Change
> Agent ID: {{.Agent.ID}}
> Hostname: {{.Agent.Hostname}}
> Interface: {{.InterfaceName}}
> IP Version: {{.Version}}
{{if .OldAddrs}}> Old IPs: {{join .OldAddrs ", "}}{{end}}
{{if .NewAddrs}}> New IPs: {{join .NewAddrs ", "}}{{end}}
{{end}}

_Changed at: {{.Timestamp | formatTime}}_
