### Network Errors Alert

**Agent ID:** {{.AgentID}}
**Interface:** {{.Interface.Name}} ({{.Interface.Type}})

#### Error Statistics

- RX Errors: {{.Interface.Statistics.RxErrors}}
- TX Errors: {{.Interface.Statistics.TxErrors}}
- RX Dropped: {{.Interface.Statistics.RxDropped}}
- TX Dropped: {{.Interface.Statistics.TxDropped}}

> Please check the network interface.
