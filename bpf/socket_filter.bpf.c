// +build ignore

#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/in.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

// Event structure passed from Kernel eBPF probe to Go User-Space daemon via RingBuffer
struct span_event {
    __u32 src_ip;
    __u32 dst_ip;
    __u16 src_port;
    __u16 dst_port;
    __u32 payload_len;
    char  payload[256];
};

// eBPF RingBuffer Map definition
struct {
    __uint(type, BPF_MAP_TYPE_RINGBUF);
    __uint(max_entries, 256 * 1024); // 256KB RingBuffer
} events SEC(".maps");

// Socket Filter Program attached to Linux Kernel Socket Layer
SEC("socket")
int traceguard_socket_filter(struct __sk_buff *skb) {
    struct iphdr ip;
    struct tcphdr tcp;

    // 1. Read IP Header from packet buffer
    if (bpf_skb_load_bytes(skb, 0, &ip, sizeof(ip)) < 0) {
        return 0;
    }

    // 2. Filter: Only inspect TCP packets
    if (ip.protocol != IPPROTO_TCP) {
        return 0;
    }

    // 3. Read TCP Header
    if (bpf_skb_load_bytes(skb, sizeof(struct iphdr), &tcp, sizeof(tcp)) < 0) {
        return 0;
    }

    __u16 dst_port = bpf_ntohs(tcp.dest);
    __u16 src_port = bpf_ntohs(tcp.source);

    // 4. Filter: Only inspect application & database ports (8080 HTTP, 5432 Postgres, 6379 Redis)
    if (dst_port != 8080 && dst_port != 5432 && dst_port != 6379 &&
        src_port != 8080 && src_port != 5432 && src_port != 6379) {
        return 0;
    }

    // 5. Reserve space in RingBuffer
    struct span_event *event = bpf_ringbuf_reserve(&events, sizeof(struct span_event), 0);
    if (!event) {
        return 0;
    }

    event->src_ip = ip.saddr;
    event->dst_ip = ip.daddr;
    event->src_port = src_port;
    event->dst_port = dst_port;

    // 6. Copy packet payload bytes (HTTP headers / SQL query text)
    __u32 offset = sizeof(struct iphdr) + sizeof(struct tcphdr);
    bpf_skb_load_bytes(skb, offset, event->payload, sizeof(event->payload));

    // 7. Submit event to Go User-Space daemon
    bpf_ringbuf_submit(event, 0);

    return 0;
}

char _license[] SEC("license") = "GPL";
