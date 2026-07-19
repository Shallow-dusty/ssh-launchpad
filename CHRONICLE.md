# Project Chronicle

`STATUS.md` owns current product facts and `CHANGELOG.md` owns user-visible
version changes. This file records origin, responsibility changes, and project
promotion without duplicating either document.

## 2026-07-18 - Single implementation established

- Combined the reusable parts of the former `00.scripts/01.SSH快速安装` kit and
  the 3070 remote-control workspace into one independent repository.
- Kept device profiles, device evidence, and incidents in their device
  workspace; this repository became the only generic implementation.
- Published `v0.1.0` with the cross-platform CLI, bootstrap scripts, Windows
  installer, checksums, and SBOM.

## 2026-07-19 - Beginner MVP and standalone promotion

- Published `v0.2.0` with the Chinese/English desktop and CLI wizards,
  double-click launchers, key-role onboarding, self-cut protection, offline
  help, portable bundles, and expanded release verification.
- Passed the MVP promotion gate and moved the canonical local checkout from
  `E:\coding\01.Agent-CLI\15.SSH-Launchpad` to
  `E:\coding\11.SSH-Launchpad`.
- Retired the incubation path and kept migration residue only in the parent
  workspace archive for provenance.
