# Ansible role: filter-chain
Installs filter-chain on Debian/Ubuntu Linux servers.

## Requirements
None

## Role Variables
Available variables are listed below, along with default values (see `defaults/main.yml`):


```yaml
filter_chain_arch: amd64
```
System arch. Available `amd64`, `arm64`


```yaml
filter_chain_version: "v0.2.0"
```
Release version for installation


```yaml
filter_chain_networks_allow: []
```
List of allowed subnets. E.g. `"127.0.0.0/8"`


```yaml
filter_chain_countries_allow: []
```
List of allowed country A2 codes from [RIPE](https://www.ripe.net/participate/member-support/list-of-members/list-of-country-codes-and-rirs)


```yaml
filter_chain_countries_deny: []
```
List of denied country A2 codes from [RIPE](https://www.ripe.net/participate/member-support/list-of-members/list-of-country-codes-and-rirs)


```yaml
filter_chain_ripe_refresh: "12h"
```
Time interval between country data received from RIPE. It does not make much sense to set it less than 12 hours.

```yaml
filter_chain_drop_at_end: false
```
Boolean. If true, adds drop (`-j DROP`) rule at the end of chain.

## Dependencies
None

## Example Playbook
```yaml
---
- hosts: all
  become: true
  roles:
    - { role: filter-chain-role }
```