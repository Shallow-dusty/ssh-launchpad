# Security policy

If you find a vulnerability that could expose credentials, weaken remote
access, bypass confirmation, or interrupt a control channel, please use
GitHub's private vulnerability reporting instead of a public issue. Never
attach real private keys, tokens, or unredacted logs.

Security guarantees: Check and Plan are read-only; Verify does not elevate and
fails closed on unknown authentication/firewall evidence; Apply requires the
reviewed plan digest and blocks control-channel self-cut by default; downloads
require HTTPS and SHA-256. A violation of these guarantees is a bug — report
it privately.
