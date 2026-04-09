# Native KubeVirt Backup API Integration (PR #412)

Status: Draft
Author: @aagarwal-apexanalytix
Tracking issue: [#411](https://github.com/kubevirt/kubevirt-velero-plugin/issues/411)
PR: [#412](https://github.com/kubevirt/kubevirt-velero-plugin/pull/412)

## Summary

Add optional, opt-in support for the native KubeVirt backup API
(`backup.kubevirt.io/v1alpha1`) inside the existing `kubevirt-velero-plugin`,
without introducing a new controller or Velero core changes. Native
`VirtualMachineBackup` is used to produce a CBT/quiesced copy of VM volumes
into a scratch PVC; Velero then backs up the scratch PVC using whatever
snapshot path is already configured (CSI, fs-backup, datamover). The existing
CSI path remains the default and is used as a fallback whenever the native
path is unavailable.

**Scope caveat: backup only, no end-to-end restore.** This PR ships the
backup side. The scratch PVC contains qcow2 files (full + incrementals);
restore puts those files back on a PVC via the existing CSI restore path,
but reconstituting a usable VM disk from the qcow2 chain (`qemu-img
rebase` / `convert`) is intentionally left to the OADP kubevirt-datamover.
Users who need end-to-end native restore today should use the OADP
datamover; users who want CBT/quiesced *backups* today with upstream
Velero can use this.

This document compares the proposal to the [OADP kubevirt-datamover
design](https://github.com/openshift/oadp-operator/blob/oadp-dev/docs/design/kubevirt-datamover.md)
and the related
[migtools/kubevirt-datamover-controller](https://github.com/migtools/kubevirt-datamover-controller),
and explains why we believe these two approaches are complementary rather
than competing.

## Background

KubeVirt `v1.8` exposes a native backup API: `VirtualMachineBackup` and
`VirtualMachineBackupTracker` (`backup.kubevirt.io/v1alpha1`), built on top
of QEMU dirty bitmaps / CBT. It enables:

- Quiesced, crash-consistent snapshots when the QEMU guest agent is present
- True block-level incremental backup (only changed blocks)
- VM awareness at the hypervisor level rather than the storage/PVC level

Today, `kubevirt-velero-plugin` only integrates with Velero via CSI
`VolumeSnapshot`s. For VMs this means Velero sees PVCs, snapshots them,
and (optionally) moves data via kopia. The hypervisor is uninvolved, so
incremental backups are computed by scanning whole volumes rather than
reading the CBT.

The OADP proposal solves this by introducing a new data-mover path: a
custom `DataUpload.Spec.DataMover=kubevirt`, a new
`kubevirt-datamover-controller` that reconciles those DataUploads, and a
custom object-storage layout for qcow2 checkpoints. That design is the
"full" native-backup story and requires changes to OADP, a new controller,
and small changes to Velero.

This PR takes a much smaller step that is useful today without any of
those moving parts.

## Goals

- Let users opt into CBT/quiesced backup of VMs using only the existing
  `kubevirt-velero-plugin` binary.
- Work with any upstream Velero release (no Velero core changes).
- Work with any Velero backup backend (CSI snapshot, fs-backup, CSI
  datamover, any third-party datamover) — the plugin does not reinvent
  object-storage layout or kopia plumbing.
- Preserve existing behavior exactly when the feature is not enabled.
- Degrade gracefully: if CRDs are missing, the VM is offline, the agent is
  absent, or any native call fails, fall back to the existing CSI path.

## Non-goals

- Replacing or duplicating the OADP kubevirt-datamover.
- Defining a new on-disk layout for qcow2 / CBT data in the BSL.
- Adding a new controller or CRDs owned by this plugin.
- Implementing restore from the CBT chain. For restore, Velero restores
  the scratch PVC contents exactly as it already does for any other PVC;
  reconstituting an incremental chain is out of scope for this change and
  remains the OADP datamover's territory.

## High-level design

### Opt-in surface

All behavior is off by default. Users enable it per-backup with labels on
the Velero `Backup` object, or cluster-wide via a ConfigMap
(`kubevirt-velero-plugin-config` in `velero`). Labels always win over
ConfigMap values.

| Label | Purpose |
|-------|---------|
| `velero.kubevirt.io/native-backup` | Enable native backup for this Backup |
| `velero.kubevirt.io/incremental-backup` | Enable incremental via tracker |
| `velero.kubevirt.io/skip-quiesce` | Force skip filesystem quiesce |
| `velero.kubevirt.io/scratch-storage-class` | Override scratch PVC storage class |
| `velero.kubevirt.io/force-full-every-n` | Force full backup every N incrementals |
| `velero.kubevirt.io/native-backup-concurrency` | Max concurrent native backups |

If the label is absent, the plugin behaves exactly as it does today.

### Flow (backup)

```
   Velero Backup                 Plugin (BIA v2)                KubeVirt
   ─────────────                 ────────────────               ────────
        │                                │                         │
        │  Execute(VM)                   │                         │
        ├───────────────────────────────►│                         │
        │                                │ IsEnabled? CRDs?        │
        │                                │ VM running? agent?      │
        │                                │                         │
        │                                │ create scratch PVC      │
        │                                │ create VirtualMachine-  │
        │                                │   Backup (+tracker)     │
        │                                ├────────────────────────►│
        │                                │                         │ QEMU quiesce
        │                                │                         │ CBT → scratch PVC
        │◄──── operationID, extras ──────┤                         │
        │                                │                         │
        │  Progress(opID)                │                         │
        ├───────────────────────────────►│  poll VMB status        │
        │                                ├────────────────────────►│
        │◄──── progress% / done ─────────┤                         │
        │                                │                         │
        │ (Velero snapshots the scratch  │                         │
        │  PVC like any other PVC — via  │                         │
        │  its normal CSI/datamover      │                         │
        │  path)                         │                         │
        │                                │                         │
        │  (on backup delete)            │                         │
        ├───────────────────────────────►│ DeleteItemAction cleans │
        │                                │ up VMB + scratch PVC    │
```

Key points:

- The source PVCs are labelled `velero.kubevirt.io/native-backed=true`
  so the existing PVC BIA (`pvc_backup_item_action.go`) skips them — no
  double-snapshot on the source.
- The scratch PVC is tagged with
  `velero.kubevirt.io/scratch-for-backup=<vmb-name>` and a
  `velero.kubevirt.io/scratch-pvc-ttl` annotation; Velero picks it up
  through its normal PVC path.
- `VirtualMachineBackupTracker` is used to carry checkpoint state for
  incremental runs; `force-full-every-n` bounds the chain length.
- If any native step fails (CRD missing, VM offline, VMB error, timeout),
  the plugin deletes any scratch state it created and returns without
  setting the "native-backed" label, so Velero's normal CSI path runs.
- Annotations are applied to the VM's `unstructured` item (not a decoded
  `VirtualMachine` struct), so they are persisted into the backup tarball.

### Flow (restore)

No new restore-side behavior. The scratch PVC is restored normally by
Velero/CSI; the VMB/VMBT CRs are filtered out of the restore (they would
otherwise trigger a new backup). Reconstructing a CBT chain into a usable
VM disk is intentionally left to the OADP datamover path.

### Components touched

- New package `pkg/util/nativebackup/` (~1.2k LoC production + ~340 LoC
  tests) — config, detection, scratch PVC, VMB/tracker lifecycle,
  progress, agent check.
- `vm_backup_item_action.go` upgraded to BIA v2 (`Execute` / `Progress` /
  `Cancel`) so native backup can run asynchronously.
- `vm_restore_item_action.go` upgraded to RIA v2 for symmetry.
- New `vm_delete_item_action.go` to clean up VMB + scratch PVCs on backup
  deletion.
- New `vm_item_block_action.go` so VM + scratch PVC + related resources
  are backed up atomically.
- RBAC for `backup.kubevirt.io` resources.
- Unit tests for the new package and the v2 actions.

## Comparison to OADP kubevirt-datamover

| Dimension | OADP kubevirt-datamover | This PR |
|---|---|---|
| Scope | Full data-mover replacement | Plugin-only feature |
| New controller | Yes (`kubevirt-datamover-controller`) | No |
| Velero core changes | Yes (volume policy, `SnapshotType`) | None |
| Ships where | OADP / migtools fork | Upstream `kubevirt-velero-plugin` |
| BSL layout | Custom qcow2 directory tree | Unchanged — uses whatever Velero already does |
| Storage path | Direct object-store writes from datamover pod | Scratch PVC → Velero's existing CSI/datamover |
| Incremental metadata | Per-VM `index.json` in BSL | `VirtualMachineBackupTracker` in cluster + annotations |
| Restore | Custom datamover restore pod + `qemu-img rebase` | Standard CSI restore of scratch PVC |
| Target user | OADP/OpenShift user wanting CBT end-to-end | Upstream KubeVirt + Velero user wanting CBT quiesce today |
| Upgrade cost | New CRDs, new controller, new BSL content | Add label to an existing Backup |

### Why both can coexist

- The OADP design is the "right" long-term answer for CBT-native
  end-to-end backup (including true incremental restore). It is also a
  larger change, owned by OADP, and depends on Velero core work landing.
- This PR is a short-lever improvement that any KubeVirt + Velero user
  (not just OADP users) can turn on today. It uses the native API for the
  parts where it matters most — quiesce, CBT, VM-aware snapshot — and
  lets Velero's existing machinery do data movement.
- Because this PR is strictly additive and defaults-off, it does not block
  or contradict the OADP datamover landing later. A future user of the
  OADP datamover would simply leave these labels unset, and this plugin
  stays out of the way.
- If maintainers prefer, the feature can later be retargeted to produce
  `DataUpload`s with `Spec.DataMover=kubevirt` once the OADP/Velero side
  lands, reusing the same `pkg/util/nativebackup/` primitives.

### Why not just put this in the migtools fork

- `migtools/kubevirt-velero-plugin` is currently ~14 commits ahead of
  upstream, and the delta is almost entirely OpenShift CI carries and
  rebase-bot merges — no native-backup implementation lives there.
- `migtools/kubevirt-datamover-controller` implements the OADP design
  above, which is a separate shape.
- Upstream users who don't run OADP still benefit from CBT/quiesce. The
  natural home for that is here.

## Open questions for maintainers

1. **Home of the feature.** Do you want the native-backup path to live
   here long-term, or do you see this as a stopgap until the OADP
   datamover lands? We are happy to scope the PR accordingly (e.g. mark
   as experimental, hide behind a build tag, or plan a migration path).

2. **Opt-in surface.** Labels on the Backup object are used today for
   ergonomics and because volume-policy support for "unrecognized
   actions" is not yet in Velero. Would you prefer a ConfigMap-only
   surface, or something else (e.g. an annotation on the VM, a Velero
   plugin config key)?

3. **Relationship to OADP DataUpload.** Is there interest in having this
   plugin optionally emit a `DataUpload` with `Spec.DataMover=kubevirt`
   (instead of the scratch-PVC approach) once the OADP controller is
   available? That would let the two efforts share one front-end.

4. **PR shape.** The current PR is size/XXL. If the design is acceptable
   in principle, we can split it into: (a) design doc, (b)
   `pkg/util/nativebackup/` package, (c) v2 action upgrades + DIA/IBA,
   (d) RBAC + config. Happy to do that.

5. **Testing.** What's the expected bar for e2e coverage of a new path
   like this? The current PR ships unit tests; we can add lane coverage
   against a KubeVirt v1.8+ cluster if CI supports it.

## Out of scope / future work

- Restore of an incremental chain (handled by OADP datamover).
- Pull-mode backup (KubeVirt enhancement, not yet stable).
- A plugin-owned object-storage layout for qcow2 files.
- A plugin-owned controller.
