# ansibleinv-digitalocean
Dynamic Ansible inventory generator written in Go as a "hello world" project.

## Inventory Hostname and IPs

Inventory hostnames are generated from the Droplet's human-readable name. `ansible_host` is then set to the Droplet's public IPv4 address.

``` bash
$ ansible -i ../../../gopath/src/github.com/digaxfr/dyn-inv-digitalocean/do all -m debug -a "msg={{ ansible_host }}"
vps2 | SUCCESS => {
    "msg": "206.81.x.abc"
}
vps3 | SUCCESS => {
    "msg": "104.248.x.abc"
}
```

## Building

``` bash
go build do.go
```

## Usage

``` bash
ansible-playbook -i /path/to/do <playbook>
```
