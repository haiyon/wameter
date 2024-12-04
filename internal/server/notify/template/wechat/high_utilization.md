## High Network Utilization

> Agent ID: {{.AgentID}}
> Interface: {{.Interface.Name}}
> Type: {{.Interface.Type}}

**Current Rates:**

- Receive: {{.Interface.Statistics.RxBytesRate | formatBytesRate}}/s
- Transmit: {{.Interface.Statistics.TxBytesRate | formatBytesRate}}/s

**Total Traffic:**

- Received: {{.Interface.Statistics.RxBytes | formatBytes}}
- Transmitted: {{.Interface.Statistics.TxBytes | formatBytes}}

_Alert generated at {{.Timestamp | formatTime}}_
