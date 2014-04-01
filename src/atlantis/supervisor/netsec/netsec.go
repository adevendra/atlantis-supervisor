package netsec

import (
	"atlantis/supervisor/containers/serialize"
	"errors"
	"sync"
)

type NetworkSecurity struct {
	sync.Mutex
	SaveFile   string                        // where to save state
	DeniedIPs  map[string]bool               // list of denied IPs. map for easy existence check
	IPGroups   map[string][]string           // group name -> list of infrastructure IPs to blanket deny
	Containers map[string]*ContainerSecurity // container id -> ContainerSecurity
}

func New(saveFile string) *NetworkSecurity {
	return &NetworkSecurity{
		Mutex:      sync.Mutex{},
		SaveFile:   saveFile,
		DeniedIPs:  map[string]bool{},
		IPGroups:   map[string][]string{},
		Containers: map[string]*ContainerSecurity{},
	}
}

func (n *NetworkSecurity) save() {
	// save state
	serialize.SaveAll(serialize.SaveDefinition{
		n.SaveFile,
		n,
	})
}

func (n *NetworkSecurity) UpdateIPGroup(name string, ips []string) error {
	n.Lock()
	defer n.Unlock()

	if err := delConnTrackRule(); err != nil {
		return err
	}

	current, exists := n.IPGroups[name]
	toRemove := []string{}
	if exists {
		// figure out what is being removed
		incomingMap := map[string]bool{}
		for _, ip := range ips {
			incomingMap[ip] = true
		}
		for _, ip := range current {
			if !incomingMap[ip] {
				toRemove = append(toRemove, ip)
			}
		}
	}
	// add blanket deny rule for new IPs
	newIPs := []string{}
	for _, ip := range ips {
		if n.DeniedIPs[ip] {
			// already exists
			continue
		}
		if err := n.rejectIP(ip); err != nil {
			return err
		}
		n.DeniedIPs[ip] = true
		newIPs = append(newIPs, ip)
	}
	// remove blanket deny rule for removed IPs
	for _, ip := range toRemove {
		if !n.DeniedIPs[ip] {
			// already removed
			continue
		}
		n.allowIP(ip)
		delete(n.DeniedIPs, ip)
	}
	// add/remove forward rules for new IPs for everything that uses the name
	for _, contSec := range n.Containers {
		ports, exists := contSec.SecurityGroups[name]
		if !exists {
			// this container does not use this ip group
			continue
		}
		for _, port := range ports {
			// add new ips
			for _, ip := range newIPs {
				contSec.allowPort(ip, port)
			}
			// remove old ips
			for _, ip := range toRemove {
				contSec.rejectPort(ip, port)
			}
		}
	}
	// update ipGroups
	n.IPGroups[name] = ips
	n.save()

	return addConnTrackRule()
}

func (n *NetworkSecurity) DeleteIPGroup(name string) error {
	if err := n.UpdateIPGroup(name, []string{}); err != nil {
		return err
	}
	n.Lock()
	defer n.Unlock()
	delete(n.IPGroups, name)
	n.save()
	return nil
}

func (n *NetworkSecurity) AddContainerSecurity(id string, pid int, sgs map[string][]uint16) error {
	n.Lock()
	defer n.Unlock()
	if _, exists := n.Containers[id]; exists {
		// we already have security set up for this id. don't do it and return an error.
		return errors.New("Container " + id + " already has Network Security set up.")
	}
	// make sure all groups exist
	for group, _ := range sgs {
		_, exists := n.IPGroups[group]
		if !exists {
			return errors.New("IP Group " + group + " does not exist")
		}
	}

	// fetch network info
	contSec, err := NewContainerSecurity(id, pid, sgs)
	if err != nil {
		return err
	}
	contSec.addMark()

	// add forward rules
	for group, ports := range sgs {
		for _, port := range ports {
			ips := n.IPGroups[group]
			for _, ip := range ips {
				if err := contSec.allowPort(ip, port); err != nil {
					defer n.RemoveContainerSecurity(id) // cleanup created references when we error out
					return err
				}
			}
		}
	}
	n.Containers[id] = contSec
	n.save()
	return nil
}

func (n *NetworkSecurity) RemoveContainerSecurity(id string) error {
	n.Lock()
	defer n.Unlock()

	contSec, exists := n.Containers[id]
	if !exists {
		// no container security here, nothing to remove
		return nil
	}

	contSec.delMark()
	// remove forward rules
	for group, ports := range contSec.SecurityGroups {
		for _, port := range ports {
			ips := n.IPGroups[group]
			for _, ip := range ips {
				contSec.rejectPort(ip, port)
			}
		}
	}
	delete(n.Containers, id)
	n.save()
	return nil
}

func delConnTrackRule() error {
	_, err := executeCommand("iptables", "-D", "FORWARD", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")
	return err
}

func addConnTrackRule() error {
	_, err := executeCommand("iptables", "-I", "FORWARD", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")
	return err
}

func (n *NetworkSecurity) forwardRule(action, ip string) error {
	_, err := executeCommand("iptables", action, "FORWARD", "-d", ip, "-j", "REJECT")
	return err
}

func (n *NetworkSecurity) rejectIP(ip string) error {
	return n.forwardRule("-I", ip)
}

func (n *NetworkSecurity) allowIP(ip string) error {
	return n.forwardRule("-D", ip)
}