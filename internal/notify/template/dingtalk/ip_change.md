### IP Address Change Detected

{{if .Change.IsExternal}}
**External IP Change**

> Agent ID: {{.Agent.ID}}
> Hostname: {{.Agent.Hostname}}
> IP Version: {{.Change.Version}}
> Old IP: {{index .Change.OldAddrs 0}}
> New IP: {{index .Change.NewAddrs 0}}
{{else}}
**Interface IP Change**

> Agent ID: {{.Agent.ID}}
> Hostname:{{.Agent.Hostname}}
> Interface: {{.Change.InterfaceName}}
> IP Version: {{.Change.Version}}
> Old IPs: {{join .Change.OldAddrs ", "}}
> New IPs: {{join .Change.NewAddrs ", "}}
{{end}}

_Changed at: {{.Change.Timestamp | formatTime}}_