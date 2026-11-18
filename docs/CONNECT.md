# Connect

1. Deploy a `SatisfactoryServer` with `gateway.serviceType: LoadBalancer` and
   `gateway.address` set to your kube-vip VIP.
2. Wait until `.status.ready` is true (first start downloads ~15Gi via SteamCMD).
3. In Satisfactory → Server Manager → Add Server →
   `.status.connectionAddress` (port `.status.connectionPort`, default 7777).
4. **Claim** the server and set a strong admin password immediately.
5. Create or load a session; friends join via the server browser or Direct Connect.

Reliable messaging uses `.status.reliablePort` (default 8888 TCP) — ensure firewall /
LoadBalancer exposes 7777 TCP+UDP and 8888 TCP.

## Envoy Gateway

`gateway.enabled: true` is experimental. Unreal UDP joins often fail behind Envoy source NAT.
Prefer a dedicated VIP on the game Service.
