package proxy

import (
	"fmt"
	"hash/fnv"
	"strings"
	"sync"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/types"
	"github.com/golang/glog"

	skymsg "github.com/skynetservices/skydns/msg"
	skyserver "github.com/skynetservices/skydns/server"
)

// TODO: return a real TTL
// TODO: support reverse lookup (currently, kube2sky doesn't set this up)
// TODO: investigate just using add/remove/update events instead of receiving
//		 the whole sevice set each time

const domainSuffix = ".cluster.local."
const srvPathFormat = "%s.%s.svc" + domainSuffix
const srvPortPathFormat = "_%s._%s." + srvPathFormat
const headlessSrvPathFormat = "%s." + srvPathFormat
const headlessSrvPortPathFormat = "%s." + srvPortPathFormat

type serviceRecord interface {
	getAllEntries() ([]skymsg.Service, bool)
	getEntriesFor(segments []string) ([]skymsg.Service, bool)
	confirm()
}

type normalServiceRecord struct {
	portRecords  map[string]skymsg.Service
	normalRecord skymsg.Service
}

func (rec *normalServiceRecord) getAllEntries() ([]skymsg.Service, bool) {
	entries := make([]skymsg.Service, 0, len(rec.portRecords)+1)
	for _, entry := range rec.portRecords {
		entries = append(entries, entry)
	}
	return append(entries, rec.normalRecord), true
}

func (rec *normalServiceRecord) getEntriesFor(segments []string) ([]skymsg.Service, bool) {
	switch len(segments) {
	case 1:
		// ns
		fallthrough
	case 2:
		// name.ns
		return rec.getAllEntries()
	case 4:
		// _port._proto.name.ns
		entry, ok := rec.portRecords[segments[0]+"."+segments[1]]
		return []skymsg.Service{entry}, ok
	default:
		// we don't support > 4 or _proto.name.ns
		return nil, false
	}
}

func (rec *normalServiceRecord) confirm() {}

type headlessServiceRecord struct {
	endpointRecords map[string]skymsg.Service
	portRecords     map[string][]skymsg.Service
	confirmed       bool
}

func (rec *headlessServiceRecord) getAllEntries() ([]skymsg.Service, bool) {
	entries := make([]skymsg.Service, 0, len(rec.portRecords)+len(rec.portRecords)*len(rec.endpointRecords))
	for _, entry := range rec.endpointRecords {
		entries = append(entries, entry)
	}
	for _, entry := range rec.portRecords {
		entries = append(entries, entry...)
	}
	return entries, true
}

func (rec *headlessServiceRecord) getEntriesFor(segments []string) ([]skymsg.Service, bool) {
	switch len(segments) {
	case 1:
		// ns
		fallthrough
	case 2:
		// name.ns
		return rec.getAllEntries()
	case 3:
		if strings.HasPrefix(segments[0], "_") {
			// _proto.name.ns
			return nil, false
		}
		// ep.name.ns
		entry, ok := rec.endpointRecords[segments[0]]
		return []skymsg.Service{entry}, ok
	case 4:
		// _port._proto.name.ns
		entries, ok := rec.portRecords[segments[0]+"."+segments[1]]
		return entries, ok
	default:
		// we don't support > 5 or _proto.name.ns
		return nil, false
	}
}

func (rec *headlessServiceRecord) confirm() {
	rec.confirmed = true
}

type DNSHandler struct {
	mu             sync.RWMutex
	serviceRecords map[types.NamespacedName]serviceRecord
	isHeadless     map[types.NamespacedName]bool
}

func NewDNSHandler() *DNSHandler {
	return &DNSHandler{
		serviceRecords: make(map[types.NamespacedName]serviceRecord),
		isHeadless:     make(map[types.NamespacedName]bool),
	}
}

type DNSServiceHandler struct {
	*DNSHandler
}

type DNSEndpointHandler struct {
	*DNSHandler
}

func getHash(text string) string {
	h := fnv.New32a()
	h.Write([]byte(text))
	return fmt.Sprintf("%x", h.Sum32())
}

func (handler *DNSServiceHandler) OnUpdate(services []api.Service) {
	activeServices := make(map[types.NamespacedName]bool)

	handler.mu.Lock()
	defer handler.mu.Unlock()

	for i := range services {
		service := &services[i]
		name := types.NamespacedName{service.Namespace, service.Name}

		if !api.IsServiceIPSet(service) {
			glog.V(3).Infof("Skipping normal DNS entry for service %s due to clusterIP = %q", name, service.Spec.ClusterIP)
			handler.isHeadless[name] = true
			if entry, ok := handler.serviceRecords[name]; ok {
				entry.confirm()
			}
			activeServices[name] = true
			continue
		}

		handler.isHeadless[name] = false

		handler.makeEntriesForService(name, service)
		activeServices[name] = true
	}

	for name := range handler.serviceRecords {
		if !activeServices[name] {
			glog.V(3).Infof("Removing DNS entries for service %q", name)
			delete(handler.serviceRecords, name)
			delete(handler.isHeadless, name)
		}
	}
}

func (handler *DNSEndpointHandler) OnUpdate(endpointSets []api.Endpoints) {
	handler.mu.Lock()
	defer handler.mu.Unlock()

	for i := range endpointSets {
		endpointSet := &endpointSets[i]
		name := types.NamespacedName{endpointSet.Namespace, endpointSet.Name}

		isHeadless, confirmed := handler.isHeadless[name]
		if confirmed && !isHeadless {
			glog.V(3).Infof("Skipping non-headless service %s", name)
			continue
		}

		handler.makeEntriesForEndpoints(name, endpointSet, confirmed)
	}
}

func serviceSubdomain(serviceName types.NamespacedName, portName string, portProto api.Protocol, headlessId string) string {
	if headlessId == "" {
		if portName == "" && portProto == "" {
			return strings.ToLower(fmt.Sprintf(srvPathFormat, serviceName.Name, serviceName.Namespace))
		} else {
			return strings.ToLower(fmt.Sprintf(srvPortPathFormat, portName, portProto, serviceName.Name, serviceName.Namespace))
		}
	} else {
		if portName == "" && portProto == "" {
			return strings.ToLower(fmt.Sprintf(headlessSrvPathFormat, headlessId, serviceName.Name, serviceName.Namespace))
		} else {
			return strings.ToLower(fmt.Sprintf(headlessSrvPortPathFormat, headlessId, portName, portProto, serviceName.Name, serviceName.Namespace))
		}
	}
}

func makeServicePortSpec(port api.ServicePort) string {
	return fmt.Sprintf("_%s._%s", port.Name, strings.ToLower(string(port.Protocol)))
}

func makeEndpointPortSpec(port api.EndpointPort) string {
	return fmt.Sprintf("_%s._%s", port.Name, strings.ToLower(string(port.Protocol)))
}

func (handler *DNSHandler) makeEntriesForService(name types.NamespacedName, service *api.Service) {
	// the Group property lets us mimic the etcd behavior, since the "shortest" result will always
	// be the "a" group, and thus only "a" records get returned unless we are asked for a SRV
	// record by the full path
	newEntry := normalServiceRecord{portRecords: make(map[string]skymsg.Service)}
	srvPath := serviceSubdomain(name, "", "", "")
	for _, port := range service.Spec.Ports {
		newEntry.portRecords[makeServicePortSpec(port)] = skymsg.Service{
			Host: srvPath,
			Port: port.Port,

			Priority: 10,
			Weight:   10,
			Ttl:      30,

			Text: "",
			Key:  skymsg.Path(serviceSubdomain(name, port.Name, port.Protocol, "")),

			Group: "srv",
		}
	}

	newEntry.normalRecord = skymsg.Service{
		Host: service.Spec.ClusterIP,
		Port: 0,

		Priority: 10,
		Weight:   10,
		Ttl:      30,

		Text: "",
		Key:  skymsg.Path(srvPath),

		Group: "a",
	}

	handler.serviceRecords[name] = &newEntry
}

func (handler *DNSHandler) makeEntriesForEndpoints(name types.NamespacedName, endpointSet *api.Endpoints, confirmed bool) {
	newEntry := headlessServiceRecord{
		portRecords:     make(map[string][]skymsg.Service),
		endpointRecords: make(map[string]skymsg.Service),
	}
	for _, endpoints := range endpointSet.Subsets {
		for _, addr := range endpoints.Addresses {
			hsh := getHash(addr.IP)
			srvSubdomain := serviceSubdomain(name, "", "", hsh)
			srvPath := skymsg.Path(srvSubdomain)
			for _, port := range endpoints.Ports {
				portSpec := makeEndpointPortSpec(port)
				newEntry.portRecords[portSpec] = append(newEntry.portRecords[portSpec], skymsg.Service{
					Host: srvSubdomain,
					Port: port.Port,

					Priority: 10,
					Weight:   10,
					Ttl:      30,

					Text: "",
					Key:  skymsg.Path(serviceSubdomain(name, port.Name, port.Protocol, hsh)),

					Group: "srv",
				})
			}

			newEntry.endpointRecords[hsh] = skymsg.Service{
				Host: addr.IP,
				Port: 0,

				Priority: 10,
				Weight:   10,
				Ttl:      30,

				Text: "",
				Key:  srvPath,

				Group: "a",
			}
		}
	}

	handler.serviceRecords[name] = &newEntry
}

func (handler *DNSHandler) getEntriesForNamespace(namespace string) ([]skymsg.Service, bool) {
	if namespace == "*" || namespace == "any" {
		return handler.getAllServiceEntries()
	}

	handler.mu.RLock()
	defer handler.mu.RUnlock()
	nsServices := []skymsg.Service{}

	for name, record := range handler.serviceRecords {
		if name.Namespace != namespace {
			continue
		}

		if services, ok := record.getAllEntries(); ok {
			nsServices = append(nsServices, services...)
		}
	}

	return nsServices, true
}

func (handler *DNSHandler) getAllServiceEntries() ([]skymsg.Service, bool) {
	handler.mu.RLock()
	defer handler.mu.RUnlock()
	allServices := []skymsg.Service{}
	for _, record := range handler.serviceRecords {
		if services, ok := record.getAllEntries(); ok {
			allServices = append(allServices, services...)
		}
	}

	return allServices, true
}

func (handler *DNSHandler) getEntriesFor(segments []string) ([]skymsg.Service, bool) {
	segLen := len(segments)
	name := types.NamespacedName{segments[segLen-1], segments[segLen-2]}

	handler.mu.RLock()
	defer handler.mu.RUnlock()

	if name.Namespace == "*" || name.Namespace == "any" {
		filteredServices := []skymsg.Service{}

		if name.Name == "*" || name.Name == "any" {
			for _, record := range handler.serviceRecords {
				if services, ok := record.getEntriesFor(segments); ok {
					filteredServices = append(filteredServices, services...)
				}
			}

			return filteredServices, true
		} else {
			for recName, record := range handler.serviceRecords {
				if recName.Name != name.Name {
					continue
				}

				if services, ok := record.getEntriesFor(segments); ok {
					filteredServices = append(filteredServices, services...)
				}
			}

			return filteredServices, true
		}
	}

	if name.Name == "*" || name.Name == "any" {
		filteredServices := []skymsg.Service{}
		for recName, record := range handler.serviceRecords {
			if recName.Namespace != name.Namespace {
				continue
			}

			if services, ok := record.getEntriesFor(segments); ok {
				filteredServices = append(filteredServices, services...)
			}
		}

		return filteredServices, true
	}

	if record, ok := handler.serviceRecords[name]; !ok {
		return nil, false
	} else {
		return record.getEntriesFor(segments)
	}
}

func (handler *DNSHandler) Records(name string, exact bool) ([]skymsg.Service, error) {
	prefix := strings.Trim(strings.TrimSuffix(name, domainSuffix), ".")
	segments := strings.Split(prefix, ".")

	segLen := len(segments)
	if segLen == 0 {
		return nil, nil
	}

	if segments[segLen-1] == "svc" {
		if len(segments) > 6 {
			return nil, nil
		}

		if exact && len(segments) < 3 {
			// for exact we need either name.ns.svc.cluster.local
			// or _port._proto.name.ns.cluster.local
			return nil, nil
		}

		var ok bool
		var services []skymsg.Service

		switch len(segments) {
		case 1:
			// return all services
			services, ok = handler.getAllServiceEntries()
		case 2:
			services, ok = handler.getEntriesForNamespace(segments[0])
		default:
			services, ok = handler.getEntriesFor(segments[0 : segLen-1])
		}

		if !ok {
			return nil, fmt.Errorf("no record(s) for '%s'", name)
		}

		return services, nil
	}

	// ignore the legacy case for the moment
	return nil, fmt.Errorf("no record(s) for '%s'", name)
}

func (handler *DNSHandler) ReverseRecord(name string) (*skymsg.Service, error) {
	return nil, fmt.Errorf("reverse lookup not supported")
}

func ServeDNS(handler *DNSHandler) error {
	config := &skyserver.Config{
		Domain: "cluster.local.",
		Local:  "dns.default.svc" + domainSuffix,
	}
	err := skyserver.SetDefaults(config)
	config.DnsAddr = ""
	config.NoRec = true
	if err != nil {
		glog.Fatalf("could not start DNS: %v", err)
	}
	dnsServer := skyserver.New(handler, config)
	skyserver.Metrics()
	return dnsServer.Run()
}
