# Require HTTPS trusts X-Forwarded-Proto only from trusted proxies

Sode serves plain HTTP and leaves TLS to the user's proxy, so it cannot see HTTPS itself. With Require HTTPS on, a request counts as secure only when it arrives from an address in `trustedProxies` carrying `X-Forwarded-Proto: https`, the same trust rule the client address uses. From anyone else the header is ignored, because anyone can send it.

Rules that follow: the setting refuses to save with an empty `trustedProxies`; a save that turns it on is refused unless that request itself arrived securely, so a missing header cannot shut you out of Settings; `-reset-https` is the console escape hatch, like `-reset-pin`; the health endpoints stay open so container probes work; HSTS is sent only while the setting is on, with a short max-age because browsers keep it after the setting goes off.

Considered: refusing on the connection alone. Sode never terminates TLS, so that would refuse everything behind a proxy.
