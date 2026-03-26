# Velero Deployment and Setup Changes (Branch: eyeroll)

Summary of all changes since `main` across 10 commits on the `eyeroll` branch. These changes upgrade core dependencies, fix CSI backup enablement, improve status polling and error reporting, and add resilience to stuck VMs and Velero pods.

---

## Commits (newest first)

| SHA | Message |
|-----|---------|
| `5b6690ce` | wait for pvc bind before backup, poll pvc status, restart velero for each test |
| `7e61d471` | update velero deployment |
| `7af32065` | DEAR LORD ENABLE CSI BACKUPS 0/ |
| `692a78fc` | fix comment |
| `aa549d02` | if vm is stuck in scheduling, restart vm |
| `0a15b9fd` | update velero deployment, eye roll |
| `4914f5a7` | fix the status polling on backup and restores |
| `4dd183ca` | Update hack/config.sh versions for Velero 1.18 and KubeVirt 1.7.1 |
| `3e9a0507` | Update builder image to Go 1.25.7 |
| `65f62862` | Bump Velero to v1.18.0, KubeVirt to v1.7.1, and k8s to v0.33.5 |

---

## 1. Version Bumps

### `go.mod`
- Go: `1.23.6` → `1.25.7`
- Velero: `v1.16.2` → `v1.18.0`
- k8s (api, apimachinery, client-go): `v0.31.3` → `v0.33.5`
- KubeVirt (api, client-go): `v1.5.0` → `v1.7.1`
- CDI API: `v1.62.0` → `v1.64.0`
- Added `github.com/kubernetes-csi/external-snapshotter/client/v7 v7.0.0`

### `Makefile`
- Builder image: `quay.io/konveyor/builder:ubi9-v1.23.6` → `ubi9-v1.25.7`

### `hack/config.sh`
- `KUBEVIRT_VERSION`: `v1.6.0` → `v1.7.1`
- `VELERO_VERSION`: `v1.16.0` → `v1.18.0`
- **Removed** `USE_CSI` and `USE_RESTIC` variables entirely (CSI is now always on, restic dropped)

---

## 2. Velero Deployment (`hack/velero/deploy-velero.sh`)

This is one of the most significant diffs. The deployment script was overhauled:

### Before
- AWS plugin `v1.10.0`
- CSI and restic were conditionally enabled via `USE_CSI` / `USE_RESTIC` env vars
- `--use-volume-snapshots=true` (used provider-native snapshots)
- Snapshot location config passed explicitly
- Waited via `_kubectl wait --for=condition=Available`
- CSI volumesnapshotclass label applied only if `USE_CSI=1`

### After
- AWS plugin upgraded from `v1.10.0` to `v1.14.0`
- **CSI always enabled** via `--features EnableCSI` (no more conditional)
- Restic support removed entirely
- `--use-volume-snapshots=false` (volume snapshots now handled by CSI, not the provider)
- Snapshot location config removed (CSI manages snapshots)
- Configurable namespace via `VELERO_NAMESPACE` env var (default: `velero`)
- Configurable volume snapshot class via `VOLUME_SNAPSHOT_CLASS` env var (default: `csi-rbdplugin-snapclass`)
- Added `--resource-timeout=20m` via JSON patch on the deployment
- Uses `_kubectl rollout status` instead of `_kubectl wait --for=condition`
- VolumeSnapshotClass label always applied

### Key diff
```diff
-PLUGINS=velero/velero-plugin-for-aws:v1.10.0
-FEATURES=""
-if [[ "${USE_CSI}" == "1" ]]; then
-  PLUGINS="${PLUGINS}"
-  FEATURES="--features=EnableCSI"
-fi
+PLUGINS=velero/velero-plugin-for-aws:v1.14.0

   ${VELERO_DIR}/velero install \
+    --namespace ${VELERO_NAMESPACE} \
     --provider aws \
     --plugins ${PLUGINS} \
-    --use-volume-snapshots=true \
+    --use-volume-snapshots=false \
+    --features EnableCSI \
     ...
-    --snapshot-location-config region=minio \
-    ${FEATURES}
-  _kubectl wait -n velero deployment/velero --for=condition=Available
-  if [[ "${USE_CSI}" == "1" ]]; then
-    _kubectl label volumesnapshotclass/csi-rbdplugin-snapclass ...
-  fi
+    --backup-location-config region=minio,s3ForcePathStyle="true",s3Url=http://minio.velero.svc:9000
+  _kubectl patch deployment velero -n velero --type='json' \
+    -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--resource-timeout=20m"}]'
+  _kubectl rollout status deployment/velero -n velero --timeout=${DEPLOYMENT_TIMEOUT}s
+  _kubectl label volumesnapshotclass/${VOLUME_SNAPSHOT_CLASS} velero.io/csi-volumesnapshot-class=true --overwrite=true
```

---

## 3. Backup/Restore Shell Script (`cmd/velero-backup-restore/velero-backup-restore.sh`)

### New: Debug detail dump functions
Two new helper functions dump `velero describe --details` and the last 50 lines of logs when a backup or restore fails:
- `dump_backup_details()`
- `dump_restore_details()`

### Backup verification: one-shot → polling loop
- **Before:** Single `velero backup get` call, checked phase once.
- **After:** Polls every 5s for up to 300s. Immediately exits on `Failed` / `PartiallyFailed` terminal states with detail dumps.

### Restore: dropped `--wait` flag, added polling
- **Before:** `velero restore create ... --wait` (blocking CLI wait), then single-shot phase check.
- **After:** `velero restore create` (non-blocking), then polls every 5s for up to 300s. Terminal failure states dump details.

### Selective restore verification tightened
- **Before:** Accepted `Completed`, `PartiallyFailed`, or `Finalizing` as success; 3-minute timeout.
- **After:** Only `Completed` or `PartiallyFailed` are accepted; `Failed` is a hard error; timeout increased to 5 minutes.

---

## 4. Test Framework (`tests/framework/`)

### `backup.go` — Restore wait logic rewritten
- **Removed** `--wait` flag from `velero restore create` CLI calls in `CreateRestoreWithLabels` and `CreateRestoreWithLabelSelector`.
- **Added** `waitForRestoreTerminalPhase()`: polls restore phase for up to 10 minutes, checking for any terminal phase (`Completed`, `PartiallyFailed`, `Failed`, `FailedValidation`).
- `WaitForRestorePhase()` now detects unexpected terminal phases early instead of timing out.
- `WaitForBackupPhase()` now dumps velero details on failure via `dumpVeleroDetails()`.
- **Added** `dumpVeleroDetails()` helper that captures `velero <entity> describe --details` and last 50 log lines.

### `framework.go` — PVC polling and Velero pod restart
- **Added** `pollPVCs()` goroutine: watches PVCs every 5s, logs every state change (NEW/UPDATE/DELETED), prints summary when all PVCs reach Bound, then slows to 10s idle polling.
- **Added** `NotifyPVCChange()`: wakes the PVC poller after operations that modify PVCs.
- Framework `BeforeEach` now starts the PVC poller and restarts the Velero pod before each test via `RestartVeleroPod()`.

### `pod.go` — Velero pod restart
- **Added** `RestartVeleroPod()`: deletes the Velero pod (label `deploy=velero`) with zero grace period, then waits up to 120s for the replacement pod to be Running with all containers ready. Clears stuck in-process state like finalizer goroutines.

### `vm.go` — Better error messages and stuck VM recovery
- **`WaitForVirtualMachineInstancePhase()`**: Extended timeout to 5 minutes. Detects VMI stuck in `Scheduling` for >1 minute and deletes the virt-launcher pod to force rescheduling. Error messages now include last observed phase.
- **`WaitForVirtualMachineStatus()`**: Error messages now include last observed status.
- **`WaitForVirtualMachineInstanceStatus()`**: Error messages now include last observed phase.
- **`WaitForVirtualMachineInstanceCondition()`**: Error messages now include a summary of all VMI conditions at timeout.
- **Added** `deleteVirtLauncherPod()`: finds the virt-launcher pod for a specific VMI by annotation and force-deletes it.

---

## 5. Test Changes

### `tests/tests_suite_test.go`
- **Added** `listPlugins()`: runs `velero plugin get` during `BeforeSuite` and prints the active plugin list for debugging.

### `tests/dv_backup_test.go`
- `EventuallyDVWith` calls updated (minor signature/usage adjustments).

### `tests/pvc_vs_labeling_test.go`
- `EventuallyDVWith` calls updated.
- Uses `framework.EventuallyDVWith` for readiness checks before backup.

### `tests/resource_filtering_test.go`
- `EventuallyDVWith` calls updated across multiple test cases.

### `tests/vm_backup_test.go`
- Minor updates to match new function signatures.

---

## Summary of Key Themes

1. **CSI is now mandatory** — no more conditional `USE_CSI` toggle; CSI snapshots are the only path.
2. **Polling replaces one-shot checks** — backup and restore verification now use retry loops with timeouts instead of single-shot status reads.
3. **`--wait` flag removed from restore CLI** — replaced with framework-level polling to avoid hangs in `Finalizing` state.
4. **Better failure diagnostics** — `velero describe --details` and logs are dumped automatically on failure.
5. **Resilience to stuck states** — VMIs stuck in `Scheduling` get their launcher pod force-deleted; Velero pod is restarted between tests to clear stale state.
6. **PVC observability** — continuous PVC status polling during tests provides real-time visibility into binding progress.
7. **Major version bumps** — Velero 1.18, KubeVirt 1.7.1, k8s 0.33.5, Go 1.25.7.
