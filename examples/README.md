# Gobi Demo Examples

Two ready to deploy Gobi setups, one designed for docker-compose and the other for kubernetes environment, are provided with the project. These examples are equivalent in funcionality, so the following descriptions apply to both.

## General description

The pipeline is: GoFlow2 --> Gobi --> Prometheus --> Grafana. Two Dashboards are provided with Grafana:

- Gobi Traffic Explorer

- Gobi Internals

The Traffic Explorer dashbord is built around the concept of monitoring traffic between AS's. This doesn't mean that Gobi can only monitor this kind of traffic; It is just this example that is bound to this target analysis. If you don't find yourself in this situation, you can install the examples anyway and build your own custom dasboard, maybe using the original one as a template.

## Minimum Requirements

Since the provided dasboard is "AS centric", there are some requirements to meet when configuring IPFIX/NetFlow exporters on network devices:

**Protocol**: NetFlow v9 or IPFIX. Other protocols are supported by GoFlow2, but only NF9 and 10 have been tested with Gobi to date.

**Required fields (Entities)**:

- Source AS number.

- Destination AS number.

- NextHop

- Protocol

- Destination Port

- Sampling rate (if statistical sampling is enabled)

The default ipv4/ipv6 template from Juniper Mx/Srx devices works out of the box. For Cisco devices with FNF exporter, each field must be manually configured.

Special care must be taken when setting flows timeout: keep them as short as possible and, in any event, these timeouts should never be greater than the Prometheus scrape interval, which in this example is 60 seconds.

The device's monitored interfaces must be at the border of the AS.

## More Information

Netflow/IPFIX streams must be sent to port 2055/udp. For the docker-compose version the destination ip is the host address. For the kubernetes version the destination ip is the address leased to the BackEnd LB service by the cluster's LoadBalancer. 

A minimum flow data rate of 500 kbit/s is configured (See the **Prometheus Exporter** section of [the project readme file](../README.md)). This setting significantly helps to keep db's cardinality low in our test environment, but may be not the best for other scenarios. Take a look at the Gobi Internals dashboard and, if necessary, try to find a balance between untracked flows and db's cardinality.

## Running Gobi

To start the docker compose version, clone this project from GitHub, change to the `gobi/examples/compose` folder and launch `docker-compose up` from the command prompt. To start the kubernetes version, apply the provided `gobi.yaml` manifest to the k8s cluster with the `kubectl` utility.

Once started, it is **highly recomended** to upload MaxMind databases to the app. Point your browser to `<GOBI_HOST_IP>:8000`, login to the FileBrowser app with admin/admin credential and upload the files `GeoLite2-ASN.mmdb` and `GeoLite2-Country.mmdb` to the home folder. Restart the docker-compose or the gobi-app pod to activate changes.

Now you can access the Grafana WebUI from `<GOBI_HOST_IP>:80`. Login credentials are admin/admin.

### Kubernetes specific

The provided manifest defines two Network Services of type LoadBalancer: one for the frontend apps (Grafana and FileBrowser) and one for the backend (GoFlow2). If you require that specific IP addresses are assigned to these services, you can add the `spec.loadBalancerIP:` directive to the LBs at the beginning of the file.

There also are some kubernetes distro that don't honor the `spec.externalTrafficPolicy: Local` Load Balancer directive. This leads to source-natting the incoming NetFlow traffic, effectivly hiding the real source devices of the NetFlow/IPFIX stream. If you find that all flows are coming from the same SamplerAddress, you are probably facing this kind of issue.