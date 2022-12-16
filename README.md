# Filter-Chain
Dynamic iptables filter chain\
If you need to block subnets from some countries for partially eliminate L7 DDOS, bots or port scan. [RIPE](https://stat.ripe.net/docs/data_api#country-resource-list) used as subnet datasource.\
To solve these problems, there are more powerful (and paid) services and I recommend to use them in the next.

## Requirements
- Any modern Linux distro with iptables and ipset support.

## Installation
Use the [**ansible**](deploy/filter-chain-role/README.md) (`deploy/filter-chain-role`) for install iptables, ipset and filter-chain.

## Configuration
Example config:
```yaml
# default []
allowNetworkList:
  - "172.16.0.0/12"

# default []
countryAllowList:
  - "NL"
  - "US"

# default []
countryDenyList:
  - "CN"

# Refresh interval for update data from RIPE
# Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
# default 12h
refreshInterval: 23h59m
```

Also you need add iptables rule with jump into `ipset-filter` chain. \
Minimal manual configuration for iptables:
```bash
iptables -I INPUT 1 -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
iptables -N ipset-filter
iptables -I INPUT 2 -m conntrack --ctstate NEW -j ipset-filter
... # Any other rules
iptables -A INPUT -i eth0 -j DROP
# Then save rules
iptables-save > /etc/iptables/rules.v4
```
Note that it is recommended to save only an empty `ipset-filter` chain. In any case, rules relating to ipsets that do not yet exist will not be applied at startup.\
In turn, the filter-chain will create ipsets at startup and add rules to the chain.

### TODO
- [X] Ansible role and install docs
- [ ] Automatic creation a jump rule
