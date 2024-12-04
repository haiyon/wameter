## Network Errors Alert

> Agent ID: {{.AgentID}}
> Interface: {{.Interface.Name}}
> Type: {{.Interface.Type}}

**Error Statistics:**

- RX Errors: {{.Interface.Statistics.RxErrors}}
- TX Errors: {{.Interface.Statistics.TxErrors}}
- RX Dropped: {{.Interface.Statistics.RxDropped}}
- TX Dropped: {{.Interface.Statistics.TxDropped}}

_Alert generated at {{.Timestamp | formatTime}}_
