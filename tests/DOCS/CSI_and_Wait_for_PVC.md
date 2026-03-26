# How PVCs Are Ensured Bound Before Running a Backup

There are **two layers** of code that ensure PVCs are bound before a backup runs: one in the **test framework** (pre-backup test setup) and one in the **plugin logic** (backup-time safety checks).

## 1. Test Framework: Waiting for DV/PVC Readiness Before Triggering Backup

In every test, before `CreateBackupForNamespace` is called, the setup code waits for the DataVolume to succeed (which implies its underlying PVC is bound). The key function is `EventuallyDVWith`:

```go
framework.EventuallyDVWith(f.KvClient, f.Namespace.Name, dvName, 180, HaveSucceeded())
```

The `EventuallyDVWith` function in `tests/framework/storage.go` does two things:

- Waits for the DataVolume to reach `Succeeded` phase (or be garbage collected).
- Waits for the PVC to exist with a bound volume (`pvc.Spec.VolumeName != ""`).

```go
func EventuallyDVWith(kvClient kubecli.KubevirtClient, namespace, name string, timeoutSec int, matcher gomegatypes.GomegaMatcher) {
    if !IsDataVolumeGC(kvClient) {
        Eventually(ThisDVWith(kvClient, namespace, name), timeoutSec, time.Second).Should(matcher)
        return
    }

    // wait PVC exists before making sure DV not nil to prevent
    // race of checking dv before it was even created
    Eventually(func() bool {
        pvc, err := ThisPVCWith(kvClient, namespace, name)()
        Expect(err).ToNot(HaveOccurred())
        return pvc != nil
    }, timeoutSec, time.Second).Should(BeTrue())

    // ... verifies DV garbage collection ...

    Eventually(func() bool {
        pvc, err := ThisPVCWith(kvClient, namespace, name)()
        Expect(err).ToNot(HaveOccurred())
        return pvc != nil && pvc.Spec.VolumeName != ""
    }, timeoutSec, time.Second).Should(BeTrue())
}
```

### WaitForPVCPhase

For tests that check PVC phase explicitly, there is `WaitForPVCPhase` in `tests/framework/storage.go`:

```go
func WaitForPVCPhase(clientSet *kubernetes.Clientset, namespace, name string, phase v1.PersistentVolumeClaimPhase) error {
    var pvc *v1.PersistentVolumeClaim
    err := wait.PollImmediate(pollInterval, waitTime, func() (bool, error) {
        var err error
        pvc, err = FindPVC(clientSet, namespace, name)
        if apierrs.IsNotFound(err) {
            return false, nil
        }
        if err != nil || pvc.Status.Phase != phase {
            return false, err
        }
        return true, nil
    })
    // ...
}
```

This is called with `v1.ClaimBound` in tests like `tests/vm_backup_test.go`:

```go
err = framework.WaitForPVCPhase(f.K8sClient, f.Namespace.Name, dvForPVCName, v1.ClaimBound)
```

### WaitForVirtualMachineStatus

For VM-based tests, the pattern is to wait for the VM to be running (which implicitly means its DVs/PVCs are ready):

```go
err = framework.WaitForVirtualMachineStatus(f.KvClient, f.Namespace.Name, vm.Name, kvv1.VirtualMachineStatusRunning)
Expect(err).ToNot(HaveOccurred())
```

## 2. Plugin Logic: Safety Checks at Backup Time

The plugin itself has guardrails in `pkg/plugin/vm_backup_item_action.go`. The `canBeSafelyBackedUp` function checks that a running VM's backup won't produce broken PVC snapshots:

```go
func (p *VMBackupItemAction) canBeSafelyBackedUp(vm *kvcore.VirtualMachine, backup *v1.Backup) (bool, error) {
    isRuning := vm.Status.PrintableStatus == kvcore.VirtualMachineStatusStarting ||
                vm.Status.PrintableStatus == kvcore.VirtualMachineStatusRunning
    if !isRuning {
        return true, nil
    }

    if !util.IsResourceInBackup("virtualmachineinstances", backup) {
        p.log.Info("Backup of a running VM does not contain VMI.")
        return false, nil
    }

    excluded, err := isVMIExcludedByLabel(vm)
    if err != nil {
        return false, errors.WithStack(err)
    }
    if excluded {
        p.log.Info("VM is running but VMI is not included in the backup")
        return false, nil
    }

    if !util.IsResourceInBackup("pods", backup) && util.IsResourceInBackup("persistentvolumeclaims", backup) {
        p.log.Info("Backup of a running VM does not contain Pod but contains PVC")
        return false, nil
    }

    return true, nil
}
```

This returns `false` for all cases where a backup might end up with a broken PVC snapshot.

## 3. Observability: PVC Polling During Tests

The `pollPVCs` goroutine in `tests/framework/framework.go` continuously watches PVC status during test execution and logs state changes. It tracks when all PVCs reach `ClaimBound`:

```go
// pollPVCs watches PVCs in the test namespace. It logs every state change and,
// once all PVCs are Bound, prints a summary and goes quiet.
func pollPVCs(ctx context.Context, client *kubernetes.Clientset, namespace string, notify <-chan struct{}) {
    // ...
    for _, cs := range curr {
        if cs.phase != string(v1.ClaimBound) {
            nowAllBound = false
            break
        }
    }
    if nowAllBound {
        allBound = true
        fmt.Fprintf(ginkgo.GinkgoWriter, "[PVC-poll] All %d PVCs Bound in ns=%s:\n", len(curr), namespace)
        // ...
    }
}
```

This is purely observational/diagnostic and does **not** gate the backup, but it provides visibility into PVC binding progress.

## Summary

The tests ensure PVCs are bound before backup by calling:

- `EventuallyDVWith(..., HaveSucceeded())` — DV succeeded = PVC bound
- `WaitForPVCPhase(..., ClaimBound)` — explicit PVC phase check
- `WaitForVirtualMachineStatus(..., Running)` — VM running implies DVs/PVCs are ready

before invoking `CreateBackupForNamespace`.

The plugin itself adds a second safety net via `canBeSafelyBackedUp` which rejects backups of running VMs when the backup configuration could lead to broken PVC snapshots.

There is **no single centralized gate** that blocks all backups until PVCs are bound — it is the responsibility of each test to wait for the appropriate readiness state before triggering the backup.
