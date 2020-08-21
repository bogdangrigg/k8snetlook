package k8snetlook

import (
	"fmt"
	"net"
	"net/http"

	"github.com/sarun87/k8snetlook/netutils"
)

func RunGatewayConnectivityCheck(checkCounter *int) {
	pass, err := netutils.SendRecvICMPMessage(Cfg.HostGatewayIP, 64, true)
	if err != nil {
		fmt.Printf("  (Failed) Error running RunGatewayConnectivityCheck. Error: %v\n", err)
		return
	}
	if pass == 0 {
		*checkCounter++
		fmt.Println("  (Passed) Gateway connectivity check completed successfully")
	} else {
		fmt.Println("  (Failed) Gateway connectivity check failed")
	}
}

func RunDstConnectivityCheck(dstIP string, checkCounter *int) {
	pass, err := netutils.SendRecvICMPMessage(dstIP, 64, true)
	if err != nil {
		fmt.Printf("  (Failed) Error running connectivity check to %s. Error: %v\n", dstIP, err)
		return
	}
	if pass == 0 {
		*checkCounter++
		fmt.Printf("  (Passed) Connectivity check to destination %s completed successfully\n", dstIP)
	} else {
		fmt.Printf("  (Failed) Connectivity check to destination %s failed\n", dstIP)
	}
}

func RunKubeAPIServiceIPConnectivityCheck(checkCounter *int) {
	// TODO: Handle secure/non-secure api-servers
	// HTTP 401 return code is a successful check
	url := fmt.Sprintf("https://%s:%d", Cfg.KubeAPIService.IP, Cfg.KubeAPIService.Port)
	var body []byte
	responseCode, err := netutils.SendRecvHTTPMessage(url, "", &body)
	if err != nil {
		fmt.Printf("  (Failed) Error running RunKubeAPIServiceIPConnectivityCheck. Error: %v\n", err)
		return
	}
	if responseCode == http.StatusUnauthorized {
		fmt.Println("  (Passed) Kube API Service IP connectivity check completed successfully")
	} else {
		fmt.Println("  (Passed) Kube API Service IP connectivity check returned a non 401 HTTP Code")
	}
	*checkCounter++
}

func RunKubeAPIEndpointIPConnectivityCheck(checkCounter *int) {
	// TODO: Handle secure/non-secure api-servers
	// HTTP 401 return code is a successful check
	endpoints := getEndpointsFromService("default", "kubernetes")
	passedCount := 0
	totalCount := len(endpoints)
	for _, ep := range endpoints {
		url := fmt.Sprintf("https://%s:%d", ep.IP, ep.Port)
		fmt.Printf("  checking endpoint: %s ........", url)
		var body []byte
		responseCode, err := netutils.SendRecvHTTPMessage(url, "", &body)
		if err != nil {
			fmt.Printf("    failed connectivity check. Error: %v\n", err)
			continue
		}
		if responseCode == http.StatusUnauthorized {
			fmt.Println("    passed connectivity check")
		} else {
			fmt.Println("    passed connectivity check. Retured non 401 code though")
		}
		passedCount++
	}
	if passedCount == totalCount {
		fmt.Println("  (Passed) Kube API Endpoint IP connectivity check")
		*checkCounter++
	} else {
		fmt.Println("  (Failed) Kube API Endoint IP connectivity check for one or more endpoints")
	}
}

func RunAPIServerHealthCheck(checkCounter *int) {
	url := fmt.Sprintf("https://%s:%d/livez?verbose", Cfg.KubeAPIService.IP, Cfg.KubeAPIService.Port)
	svcAccountToken, err := getSvcAccountToken()
	if err != nil {
		fmt.Println("  (Failed) ", err)
		return
	}
	var body []byte
	responseCode, err := netutils.SendRecvHTTPMessage(url, svcAccountToken, &body)
	if err != nil {
		fmt.Printf("    Unable to fetch api server check. Error: %v\n", err)
		return
	}
	if responseCode != http.StatusOK {
		fmt.Printf("  (Failed) status check returned non-200 http code of %d\n", responseCode)
	} else {
		fmt.Printf("%s", body)
		fmt.Println("  (Passed) please check above statuses for (ok)")
		*checkCounter++
	}
}

func RunK8sDNSLookupCheck(dnsServerIP, dstSvcName, dstSvcNamespace, dstSvcExpectedIP string, checkCounter *int) {
	dnsServerURL := fmt.Sprintf("%s:53", dnsServerIP)
	// TODO: Fetch domain information from cluster
	svcfqdn := fmt.Sprintf("%s.%s.svc.cluster.local.", dstSvcName, dstSvcNamespace)
	ips, err := netutils.RunDNSLookupUsingCustomResolver(dnsServerURL, svcfqdn)
	if err != nil {
		fmt.Printf("  (Failed) Unable to run dns lookup to %s, error: %v\n", svcfqdn, err)
		return
	}
	// Check if the resolved IP matches with the IP reported by K8s
	for _, ip := range ips {
		if ip == dstSvcExpectedIP {
			*checkCounter++
			fmt.Printf("  (Passed) dns lookup to %s returned: %v. Expected: %s\n", svcfqdn, ips, ip)
			return
		}
	}
	fmt.Printf("  (Failed) Lookup of %s retured: %v, expected: %s\n", svcfqdn, ips, dstSvcExpectedIP)
	return
}

func RunMTUProbeToDstIPCheck(dstIP string, checkCounter *int) {
	supportedMTU, err := netutils.PMTUProbeToDestIP(dstIP)
	if err != nil {
		fmt.Printf("   (Failed) Unable to run pmtud for %s. Error: %v\n", dstIP, err)
		return
	}
	fmt.Printf("   Maximum MTU that works for destination IP: %s is %d\n", dstIP, supportedMTU)
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("   Unable to fetch network interfaces. Error: %v\n", err)
		return
	}
	for _, iface := range ifaces {
		// If loopback device, skip
		if iface.Flags&net.FlagLoopback == net.FlagLoopback {
			continue
		}
		if iface.MTU > supportedMTU {
			fmt.Printf("  Iface %s has higher mtu than supported path mtu. Has: %d, should be less than %d\n", iface.Name, iface.MTU, supportedMTU)
		}
	}
	*checkCounter++
	// TODO: Check for the outgoing interface mtu and compare
	fmt.Printf("   (Passed) MTU looks good.. Retured MTU: %d\n", supportedMTU)
}