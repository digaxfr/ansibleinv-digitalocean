# ansibleinv-digitalocean
Dynamic Ansible inventory generator written in Go as a "hello world" project.

## Inventory Hostname and IPs

Inventory hostnames are generated from the Droplet's human-readable name. `ansible_host` is then set to the Droplet's public IPv4 address.

## Building

``` bash
go build do.go
```

## Usage

``` bash
ansible-playbook -i /path/to/do <playbook>
```
